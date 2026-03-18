package service

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/guyuxiang/projectc-ethereum-connector/pkg/config"
	"github.com/guyuxiang/projectc-ethereum-connector/pkg/log"
	"github.com/guyuxiang/projectc-ethereum-connector/pkg/models"
	"github.com/guyuxiang/projectc-ethereum-connector/pkg/mysql"
	"github.com/guyuxiang/projectc-ethereum-connector/pkg/store"
)

type SubscriptionService interface {
	AddTx(req models.TxSubscribeRequest)
	RemoveTx(txCode string)
	AddAddress(req models.AddressSubscribeRequest)
	RemoveAddress(req models.AddressSubscribeCancelRequest)
	Refresh(ctx context.Context) error
}

type subscriptionService struct {
	eth EthereumService
}

func NewSubscriptionService(eth EthereumService) SubscriptionService {
	return &subscriptionService{eth: eth}
}

func (s *subscriptionService) AddTx(req models.TxSubscribeRequest) {
	networkCode := configuredNetworkCode()
	code := fmt.Sprintf("%s_%s", networkCode, req.TxCode)
	row := store.TxSubscriptionPO{
		Code:         code,
		TxCode:       req.TxCode,
		NetworkCode:  networkCode,
		EndTimestamp: req.SubscribeRange.EndTimestamp,
		Status:       "ACTIVE",
	}
	mysql.DB().Where("code = ?", code).Assign(row).FirstOrCreate(&row)
}

func (s *subscriptionService) RemoveTx(txCode string) {
	networkCode := configuredNetworkCode()
	mysql.DB().Model(&store.TxSubscriptionPO{}).Where("network_code = ? and tx_code = ?", networkCode, txCode).Update("status", "CANCELLED")
}

func (s *subscriptionService) AddAddress(req models.AddressSubscribeRequest) {
	networkCode := configuredNetworkCode()
	code := fmt.Sprintf("%s_%s", networkCode, req.Address)
	row := store.AddressSubscriptionPO{
		Code:             code,
		Address:          req.Address,
		NetworkCode:      networkCode,
		StartBlockNumber: req.SubscribeRange.StartBlockNumber,
		EndBlockNumber:   req.SubscribeRange.EndBlockNumber,
		NextBlockNumber:  req.SubscribeRange.StartBlockNumber,
		Status:           "ACTIVE",
	}
	mysql.DB().Where("code = ?", code).Assign(map[string]interface{}{
		"end_block_number": req.SubscribeRange.EndBlockNumber,
		"status":           "ACTIVE",
	}).FirstOrCreate(&row)
}

func (s *subscriptionService) RemoveAddress(req models.AddressSubscribeCancelRequest) {
	networkCode := configuredNetworkCode()
	mysql.DB().Model(&store.AddressSubscriptionPO{}).
		Where("network_code = ? and address = ?", networkCode, req.Address).
		Updates(map[string]interface{}{"end_block_number": req.EndBlockNumber, "status": "ACTIVE"})
}

func (s *subscriptionService) Refresh(ctx context.Context) error {
	if err := s.expireTxSubscriptions(); err != nil {
		return err
	}
	if err := s.refreshAddressSubscriptions(ctx); err != nil {
		return err
	}
	if err := s.refreshTxSubscriptions(ctx); err != nil {
		return err
	}
	if err := s.retryTxCallbacks(ctx); err != nil {
		return err
	}
	if err := s.checkTxCallbacks(ctx); err != nil {
		return err
	}
	return s.checkAddressSyncs(ctx)
}

func (s *subscriptionService) refreshTxSubscriptions(ctx context.Context) error {
	var rows []store.TxSubscriptionPO
	if err := mysql.DB().Where("status = ? and end_timestamp >= ?", "ACTIVE", time.Now().UnixMilli()).Find(&rows).Error; err != nil {
		return err
	}
	for _, row := range rows {
		resp, err := s.eth.QueryTransaction(ctx, row.TxCode)
		if err != nil || resp == nil || !resp.IfTxOnchain || resp.Tx == nil || resp.Tx.BlockNumber == 0 {
			continue
		}
		payload, _ := json.Marshal(map[string]interface{}{
			"tx":       resp.Tx,
			"txEvents": resp.TxEvents,
		})
		payloadHash := hashPayload(payload)
		record := store.TxCallbackRecordPO{
			Code:              row.Code,
			TxCode:            row.TxCode,
			NetworkCode:       row.NetworkCode,
			Payload:           string(payload),
			PayloadHash:       payloadHash,
			Status:            "PENDING",
			CheckStatus:       "WAITING",
			ConfirmBlockCount: defaultTxConfirmBlockCount,
			NextRetryAt:       time.Now().UnixMilli(),
		}
		var existing store.TxCallbackRecordPO
		callbackExists := mysql.DB().Where("code = ?", record.Code).First(&existing).Error == nil
		if callbackExists {
			continue
		}
		if err = mysql.DB().Create(&record).Error; err != nil {
			continue
		}
		if err = deliverTxCallback(&record); err == nil {
			row.Status = "COMPLETED"
			_ = mysql.DB().Save(&row).Error
		}
		_ = mysql.DB().Save(&record).Error
	}
	return nil
}

func (s *subscriptionService) expireTxSubscriptions() error {
	return mysql.DB().
		Model(&store.TxSubscriptionPO{}).
		Where("status = ? and end_timestamp < ?", "ACTIVE", time.Now().UnixMilli()).
		Update("status", "EXPIRED").Error
}

func (s *subscriptionService) retryTxCallbacks(ctx context.Context) error {
	var rows []store.TxCallbackRecordPO
	if err := mysql.DB().
		Where("status = ? and next_retry_at <= ?", "PENDING", time.Now().UnixMilli()).
		Find(&rows).Error; err != nil {
		return err
	}
	for _, row := range rows {
		_ = ctx
		_ = deliverTxCallback(&row)
		_ = mysql.DB().Save(&row).Error
	}
	return nil
}

func (s *subscriptionService) checkTxCallbacks(ctx context.Context) error {
	var rows []store.TxCallbackRecordPO
	if err := mysql.DB().
		Where("status = ? and check_status = ?", "DELIVERED", "WAITING").
		Find(&rows).Error; err != nil {
		return err
	}
	for _, row := range rows {
		resp, err := s.eth.QueryTransaction(ctx, row.TxCode)
		if err != nil {
			continue
		}
		latest, latestErr := s.eth.GetLatestBlock(ctx)
		if latestErr != nil || latest == nil {
			continue
		}
		if resp == nil || !resp.IfTxOnchain || resp.Tx == nil {
			row.Status = "CANCELLED"
			row.CheckStatus = "DONE"
			row.LastError = "transaction not found during callback check"
			cancelPayload, _ := json.Marshal(map[string]interface{}{
				"cancelled": true,
				"tx":        map[string]interface{}{"code": row.TxCode, "networkCode": row.NetworkCode},
			})
			_ = publishCancelCallback(cancelPayload)
			_ = mysql.DB().Save(&row).Error
			continue
		}
		if latest.BlockNumber < resp.Tx.BlockNumber+row.ConfirmBlockCount {
			continue
		}
		payload, _ := json.Marshal(map[string]interface{}{
			"tx":       resp.Tx,
			"txEvents": resp.TxEvents,
		})
		payloadHash := hashPayload(payload)
		if payloadHash != row.PayloadHash {
			row.Payload = string(payload)
			row.PayloadHash = payloadHash
			row.Status = "PENDING"
			row.NextRetryAt = time.Now().UnixMilli()
			row.LastError = "payload changed after confirmation check"
			_ = deliverTxCallback(&row)
			_ = mysql.DB().Save(&row).Error
			continue
		}
		row.CheckStatus = "DONE"
		row.LastError = ""
		_ = mysql.DB().Save(&row).Error
	}
	return nil
}

func (s *subscriptionService) refreshAddressSubscriptions(ctx context.Context) error {
	var rows []store.AddressSubscriptionPO
	if err := mysql.DB().Where("status = ?", "ACTIVE").Find(&rows).Error; err != nil {
		return err
	}
	for _, row := range rows {
		latest, err := s.eth.GetLatestBlock(ctx)
		if err != nil {
			continue
		}
		if latest == nil {
			continue
		}

		end := row.EndBlockNumber
		if end > latest.BlockNumber {
			end = latest.BlockNumber
		}
		// When we have already caught up to the current head but the subscription
		// itself is open-ended, keep it ACTIVE and wait for newer blocks.
		if row.NextBlockNumber > end {
			if row.EndBlockNumber <= latest.BlockNumber {
				row.Status = "COMPLETED"
				_ = mysql.DB().Save(&row).Error
			}
			continue
		}
		nextEnd := row.NextBlockNumber + 100
		if nextEnd > end {
			nextEnd = end
		}
		discoveredTxCodes, err := s.collectSubscribedLogTxs(ctx, row.Address, row.NextBlockNumber, nextEnd)
		if err != nil {
			continue
		}
		for txCode := range discoveredTxCodes {
			s.AddTx(models.TxSubscribeRequest{
				TxCode: txCode,
				SubscribeRange: models.TxSubscribeRange{
					EndTimestamp: time.Now().Add(24 * time.Hour).UnixMilli(),
				},
			})
		}
		if nextEnd >= row.NextBlockNumber {
			s.createAddressSyncWaitingCheck(row.NetworkCode, row.Address, row.NextBlockNumber, nextEnd, discoveredTxCodes)
		}
		if nextEnd >= end {
			row.Status = "COMPLETED"
		}
		row.NextBlockNumber = nextEnd + 1
		if row.EndBlockNumber > latest.BlockNumber {
			row.Status = "ACTIVE"
		}
		_ = mysql.DB().Save(&row).Error
	}
	return nil
}

func (s *subscriptionService) createAddressSyncWaitingCheck(networkCode, address string, startBlock, endBlock uint64, txCodes map[string]struct{}) {
	if endBlock < startBlock {
		return
	}
	addresses, _ := json.Marshal([]string{strings.ToLower(address)})
	txCodeList := make([]string, 0, len(txCodes))
	for txCode := range txCodes {
		txCodeList = append(txCodeList, txCode)
	}
	txPayload, _ := json.Marshal(txCodeList)
	code := fmt.Sprintf("%s_%d_%d_%s", networkCode, startBlock, endBlock, strings.ToLower(address))
	row := store.AddressSyncWaitingCheckPO{
		Code:              code,
		NetworkCode:       networkCode,
		StartBlockNumber:  startBlock,
		EndBlockNumber:    endBlock,
		AddressSet:        string(addresses),
		TxCodeSet:         string(txPayload),
		CheckStatus:       "WAITING",
		ConfirmBlockCount: defaultAddressConfirmBlockCount,
	}
	mysql.DB().Where("code = ?", code).Assign(map[string]interface{}{
		"network_code":        row.NetworkCode,
		"start_block_number":  row.StartBlockNumber,
		"end_block_number":    row.EndBlockNumber,
		"address_set":         row.AddressSet,
		"tx_code_set":         row.TxCodeSet,
		"check_status":        row.CheckStatus,
		"confirm_block_count": row.ConfirmBlockCount,
	}).FirstOrCreate(&row)
}

func (s *subscriptionService) checkAddressSyncs(ctx context.Context) error {
	var rows []store.AddressSyncWaitingCheckPO
	if err := mysql.DB().Where("check_status = ?", "WAITING").Find(&rows).Error; err != nil {
		return err
	}
	for _, row := range rows {
		latest, err := s.eth.GetLatestBlock(ctx)
		if err != nil || latest == nil {
			continue
		}
		if latest.BlockNumber < row.EndBlockNumber+row.ConfirmBlockCount {
			continue
		}

		addresses := decodeJSONStringArray(row.AddressSet)
		existing := stringSetFromSlice(decodeJSONStringArray(row.TxCodeSet))
		latestSet, scanErr := s.scanAddressTxs(ctx, row.NetworkCode, addresses, row.StartBlockNumber, row.EndBlockNumber)
		if scanErr != nil {
			continue
		}
		for txCode := range latestSet {
			if _, ok := existing[txCode]; ok {
				continue
			}
			s.AddTx(models.TxSubscribeRequest{
				TxCode: txCode,
				SubscribeRange: models.TxSubscribeRange{
					EndTimestamp: time.Now().Add(24 * time.Hour).UnixMilli(),
				},
			})
		}
		row.CheckStatus = "DONE"
		_ = mysql.DB().Save(&row).Error
	}
	return nil
}

func (s *subscriptionService) scanAddressTxs(ctx context.Context, networkCode string, addresses []string, startBlock, endBlock uint64) (map[string]struct{}, error) {
	result := make(map[string]struct{})
	if len(addresses) == 0 || endBlock < startBlock {
		return result, nil
	}
	for _, address := range addresses {
		txSet, err := s.collectSubscribedLogTxs(ctx, address, startBlock, endBlock)
		if err != nil {
			return nil, err
		}
		for txCode := range txSet {
			result[txCode] = struct{}{}
		}
	}
	return result, nil
}

func (s *subscriptionService) collectSubscribedLogTxs(ctx context.Context, address string, startBlock, endBlock uint64) (map[string]struct{}, error) {
	result := make(map[string]struct{})
	if endBlock < startBlock {
		return result, nil
	}
	logs, err := s.eth.QueryLogs(ctx, address, startBlock, endBlock)
	if err != nil {
		return nil, err
	}
	for _, logEntry := range logs {
		eventType, eventName, _ := s.eth.DecodeLogEvent(logEntry)
		if eventType == "" && eventName == "" {
			continue
		}
		if logEntry.TxHash == "" {
			continue
		}
		result[logEntry.TxHash] = struct{}{}
	}
	return result, nil
}

func decodeJSONStringArray(raw string) []string {
	if raw == "" {
		return nil
	}
	var items []string
	if err := json.Unmarshal([]byte(raw), &items); err != nil {
		return nil
	}
	return items
}

func stringSetFromSlice(values []string) map[string]struct{} {
	result := make(map[string]struct{}, len(values))
	for _, value := range values {
		result[value] = struct{}{}
	}
	return result
}

const defaultTxConfirmBlockCount = 3
const defaultAddressConfirmBlockCount = 3

func deliverTxCallback(row *store.TxCallbackRecordPO) error {
	err := publishTxCallback([]byte(row.Payload))
	if err != nil {
		row.Status = "PENDING"
		row.RetryCount++
		row.NextRetryAt = time.Now().Add(backoffForRetry(row.RetryCount)).UnixMilli()
		row.LastError = err.Error()
		return err
	}
	row.Status = "DELIVERED"
	row.NextRetryAt = 0
	row.LastError = ""
	return nil
}

func backoffForRetry(retryCount uint64) time.Duration {
	if retryCount > 6 {
		retryCount = 6
	}
	return time.Duration(1<<retryCount) * time.Minute
}

func hashPayload(payload []byte) string {
	sum := sha256.Sum256(payload)
	return fmt.Sprintf("%x", sum[:])
}

func publishTxCallback(body []byte) error {
	return publishHTTPCallback(body, callbackKindTx)
}

func publishCancelCallback(body []byte) error {
	return publishHTTPCallback(body, callbackKindRollback)
}

const (
	callbackKindTx       = "tx"
	callbackKindRollback = "rollback"
)

func publishHTTPCallback(body []byte, kind string) error {
	cfg := config.GetConfig()
	if cfg.Callback == nil {
		return fmt.Errorf("callback config is required")
	}

	targetURL := strings.TrimSpace(cfg.Callback.Txhttpurl)
	if kind == callbackKindRollback {
		targetURL = strings.TrimSpace(cfg.Callback.Rollbackhttpurl)
	}
	if targetURL == "" {
		if kind == callbackKindRollback {
			return fmt.Errorf("callback.rollbackHttpUrl is required")
		}
		return fmt.Errorf("callback.txHttpUrl is required")
	}

	log.Infof("http callback request: kind=%s url=%s body=%s", kind, targetURL, string(body))

	client := &http.Client{Timeout: 15 * time.Second}
	req, err := http.NewRequest(http.MethodPost, targetURL, strings.NewReader(string(body)))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if cfg.Callback.Username != "" || cfg.Callback.Password != "" {
		req.SetBasicAuth(cfg.Callback.Username, cfg.Callback.Password)
	}

	resp, err := client.Do(req)
	if err != nil {
		log.Errorf("http callback request failed: kind=%s url=%s err=%v", kind, targetURL, err)
		return err
	}
	defer resp.Body.Close()

	payload, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	responseBody := strings.TrimSpace(string(payload))
	log.Infof("http callback response: kind=%s url=%s status=%d body=%s", kind, targetURL, resp.StatusCode, responseBody)

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}

	return fmt.Errorf("http callback failed: status=%d body=%s", resp.StatusCode, responseBody)
}
