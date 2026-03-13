package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/guyuxiang/projectc-ethereum-connector/pkg/config"
	"github.com/guyuxiang/projectc-ethereum-connector/pkg/models"
	"github.com/guyuxiang/projectc-ethereum-connector/pkg/mysql"
	"github.com/guyuxiang/projectc-ethereum-connector/pkg/rabbitmq"
	"github.com/guyuxiang/projectc-ethereum-connector/pkg/store"
)

type OnchainRecordService interface {
	Create(kind, key string, payload interface{}) *models.OnchainRecordResponse
	CreatePrepared(record PreparedOnchainRecord) (*models.OnchainRecordResponse, error)
	Get(kind, key string) (*models.OnchainRecordResponse, error)
	Submit(ctx context.Context, code string) error
	Refresh(ctx context.Context) error
}

type onchainRecordService struct {
	mu      sync.RWMutex
	records map[string]*models.OnchainRecordResponse
}

type PreparedOnchainRecord struct {
	Code                  string
	IdempotencyKey        string
	SignerAddress         string
	OnchainType           string
	OnchainStatus         string
	RequestBusiData       string
	ResponseBusiData      string
	NetworkCode           string
	SignedTransactionData string
	RawTransactionData    string
	TxCode                string
	Nonce                 uint64
}

func NewOnchainRecordService() OnchainRecordService {
	return &onchainRecordService{
		records: map[string]*models.OnchainRecordResponse{},
	}
}

func (s *onchainRecordService) CreatePrepared(record PreparedOnchainRecord) (*models.OnchainRecordResponse, error) {
	if mysql.DB() == nil {
		resp := &models.OnchainRecordResponse{
			IdempotencyKey:   record.IdempotencyKey,
			OnchainStatus:    record.OnchainStatus,
			ResponseBusiData: record.ResponseBusiData,
			TxCode:           record.TxCode,
		}
		s.mu.Lock()
		s.records[record.OnchainType+":"+record.IdempotencyKey] = resp
		s.mu.Unlock()
		return resp, nil
	}

	row := store.OnchainRecordPO{
		Code:                  record.Code,
		IdempotencyKey:        record.IdempotencyKey,
		SignerAddress:         record.SignerAddress,
		OnchainType:           record.OnchainType,
		OnchainStatus:         record.OnchainStatus,
		RequestBusiData:       record.RequestBusiData,
		ResponseBusiData:      record.ResponseBusiData,
		NetworkCode:           record.NetworkCode,
		SignedTransactionData: record.SignedTransactionData,
		RawTransactionData:    record.RawTransactionData,
		TxCode:                record.TxCode,
		Nonce:                 record.Nonce,
	}
	if err := mysql.DB().Create(&row).Error; err != nil {
		var existing store.OnchainRecordPO
		if findErr := mysql.DB().Where("idempotency_key = ? and onchain_type = ?", record.IdempotencyKey, record.OnchainType).First(&existing).Error; findErr == nil {
			return convertOnchainRecordPO(existing), nil
		}
		return nil, err
	}
	return convertOnchainRecordPO(row), nil
}

func (s *onchainRecordService) Create(kind, key string, payload interface{}) *models.OnchainRecordResponse {
	if mysql.DB() != nil {
		record, err := s.createInDB(kind, key, payload)
		if err == nil {
			return record
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	responseData, _ := json.Marshal(payload)
	record := &models.OnchainRecordResponse{
		IdempotencyKey:   key,
		OnchainStatus:    "PENDING",
		ResponseBusiData: string(responseData),
		TxCode:           "",
	}
	s.records[kind+":"+key] = record

	message, _ := json.Marshal(map[string]interface{}{
		"type":           kind,
		"idempotencyKey": key,
		"createdAt":      time.Now().UnixMilli(),
		"payload":        payload,
	})
	_ = rabbitmq.Publish(message)

	return record
}

func (s *onchainRecordService) Get(kind, key string) (*models.OnchainRecordResponse, error) {
	if mysql.DB() != nil {
		return s.getFromDB(kind, key)
	}
	s.mu.RLock()
	defer s.mu.RUnlock()

	record, ok := s.records[kind+":"+key]
	if !ok {
		return nil, errors.New("onchain record not found")
	}
	copy := *record
	return &copy, nil
}

func (s *onchainRecordService) Submit(ctx context.Context, code string) error {
	if mysql.DB() == nil {
		return nil
	}
	var row store.OnchainRecordPO
	if err := mysql.DB().Where("code = ?", code).First(&row).Error; err != nil {
		return err
	}
	if row.OnchainStatus != "INIT" && row.OnchainStatus != "TO_BE_RESIGN" {
		return nil
	}

	network, err := findNetworkConfig(row.NetworkCode)
	if err != nil {
		return err
	}
	client, err := ethclient.DialContext(ctx, network.RPCURL)
	if err != nil {
		return err
	}
	defer client.Close()

	if err = client.SendTransaction(ctx, mustDecodeTx(row.SignedTransactionData)); err != nil {
		if isAlreadyKnownError(err) {
			row.LastError = ""
		} else if nextNonce, ok := parseNonceConflict(err); ok {
			row.OnchainStatus = "TO_BE_RESIGN"
			row.LastError = err.Error()
			row.RetryCount++
			if saveErr := mysql.DB().Save(&row).Error; saveErr != nil {
				return saveErr
			}
			return s.reSignAndSubmit(ctx, &row, nextNonce)
		} else {
			row.LastError = err.Error()
			row.RetryCount++
			_ = mysql.DB().Save(&row).Error
			return err
		}
	}
	row.OnchainStatus = "PROCESSING"
	row.LastError = ""
	addDefaultTxSubscription(row.NetworkCode, row.TxCode)
	return mysql.DB().Save(&row).Error
}

func (s *onchainRecordService) Refresh(ctx context.Context) error {
	if mysql.DB() == nil {
		return nil
	}
	var rows []store.OnchainRecordPO
	if err := mysql.DB().Where("onchain_status in ?", []string{"INIT", "PROCESSING", "TO_BE_RESIGN"}).Find(&rows).Error; err != nil {
		return err
	}
	for _, row := range rows {
		if (row.OnchainStatus == "INIT" || row.OnchainStatus == "TO_BE_RESIGN") && row.SignedTransactionData != "" {
			_ = s.Submit(ctx, row.Code)
			continue
		}
		if row.TxCode == "" {
			continue
		}
		network, err := findNetworkConfig(row.NetworkCode)
		if err != nil {
			continue
		}
		client, err := ethclient.DialContext(ctx, network.RPCURL)
		if err != nil {
			continue
		}
		receipt, err := client.TransactionReceipt(ctx, common.HexToHash(row.TxCode))
		client.Close()
		if err != nil || receipt == nil {
			continue
		}
		txData, _ := json.Marshal(models.ChainTx{
			Code:        row.TxCode,
			NetworkCode: row.NetworkCode,
			BlockNumber: receipt.BlockNumber.Uint64(),
			Status:      map[uint64]string{1: "SUCCESS", 0: "FAILED"}[receipt.Status],
		})
		row.ChainTxData = string(txData)
		if receipt.Status == 1 {
			row.OnchainStatus = "SUCCESS"
		} else {
			row.OnchainStatus = "FAILED"
		}
		row.LastError = ""
		_ = mysql.DB().Save(&row).Error
	}
	return nil
}

func (s *onchainRecordService) createInDB(kind, key string, payload interface{}) (*models.OnchainRecordResponse, error) {
	requestData, _ := json.Marshal(payload)
	code := key
	if code == "" {
		code = fmt.Sprintf("%s-%d", kind, time.Now().UnixNano())
	}

	var row store.OnchainRecordPO
	err := mysql.DB().Where("idempotency_key = ? and onchain_type = ?", key, kind).First(&row).Error
	if err == nil {
		return convertOnchainRecordPO(row), nil
	}

	row = store.OnchainRecordPO{
		Code:             code,
		IdempotencyKey:   key,
		OnchainType:      kind,
		OnchainStatus:    "INIT",
		RequestBusiData:  string(requestData),
		ResponseBusiData: string(requestData),
	}
	if err = mysql.DB().Create(&row).Error; err != nil {
		var existing store.OnchainRecordPO
		if findErr := mysql.DB().Where("idempotency_key = ? and onchain_type = ?", key, kind).First(&existing).Error; findErr == nil {
			return convertOnchainRecordPO(existing), nil
		}
		return nil, err
	}

	message, _ := json.Marshal(map[string]interface{}{
		"onchainRecordCode": row.Code,
	})
	_ = rabbitmq.Publish(message)
	return convertOnchainRecordPO(row), nil
}

func (s *onchainRecordService) getFromDB(kind, key string) (*models.OnchainRecordResponse, error) {
	var row store.OnchainRecordPO
	if err := mysql.DB().Where("idempotency_key = ? and onchain_type = ?", key, kind).First(&row).Error; err != nil {
		return nil, errors.New("onchain record not found")
	}
	return convertOnchainRecordPO(row), nil
}

func convertOnchainRecordPO(row store.OnchainRecordPO) *models.OnchainRecordResponse {
	var chainTx *models.ChainTx
	if row.ChainTxData != "" {
		var tx models.ChainTx
		if err := json.Unmarshal([]byte(row.ChainTxData), &tx); err == nil {
			chainTx = &tx
		}
	}
	return &models.OnchainRecordResponse{
		IdempotencyKey:   row.IdempotencyKey,
		OnchainStatus:    row.OnchainStatus,
		ResponseBusiData: row.ResponseBusiData,
		ChainTxData:      chainTx,
		TxCode:           row.TxCode,
	}
}

func (s *onchainRecordService) reSignAndSubmit(ctx context.Context, row *store.OnchainRecordPO, nextNonce uint64) error {
	prepared, err := s.rebuildPreparedRecord(ctx, *row, sanitizeNextNonce(nextNonce))
	if err != nil {
		row.LastError = err.Error()
		return mysql.DB().Save(row).Error
	}
	row.Code = prepared.Code
	row.SignerAddress = prepared.SignerAddress
	row.SignedTransactionData = prepared.SignedTransactionData
	row.RawTransactionData = prepared.RawTransactionData
	row.TxCode = prepared.TxCode
	row.Nonce = prepared.Nonce
	row.OnchainStatus = "INIT"
	row.LastError = ""
	if err = mysql.DB().Save(row).Error; err != nil {
		return err
	}
	return s.Submit(ctx, row.Code)
}

func (s *onchainRecordService) rebuildPreparedRecord(ctx context.Context, row store.OnchainRecordPO, nonceOverride *uint64) (*PreparedOnchainRecord, error) {
	sc := &scplusService{txSigner: newContractTxService(NewContractRegistryService()), onchain: s}
	switch row.OnchainType {
	case "NATIVE_TOKEN_CHARGE":
		var req models.BalanceChargeRequest
		if err := json.Unmarshal([]byte(row.RequestBusiData), &req); err != nil {
			return nil, err
		}
		wallet := &walletService{nonce: NewSignerNonceService(), onchain: s}
		return wallet.createNativeChargePrepared(ctx, row.NetworkCode, req, nonceOverride)
	case "SCPLUS_SEND_SETTLE":
		var req models.SettleRequest
		if err := json.Unmarshal([]byte(row.RequestBusiData), &req); err != nil {
			return nil, err
		}
		return sc.buildDttSendSettlePrepared(ctx, row.NetworkCode, req, nonceOverride)
	case "SCPLUS_AUTO_REJECT":
		var req models.AutoRejectRequest
		if err := json.Unmarshal([]byte(row.RequestBusiData), &req); err != nil {
			return nil, err
		}
		return sc.buildAutoRejectPrepared(ctx, row.NetworkCode, req, nonceOverride)
	case "SCPLUS_ONRAMP":
		var req models.InstantOnRampRequest
		if err := json.Unmarshal([]byte(row.RequestBusiData), &req); err != nil {
			return nil, err
		}
		return sc.buildInstantOnRampPrepared(ctx, row.NetworkCode, req, nonceOverride)
	case "SCPLUS_FINANCE":
		var req models.FinanceInvokeRequest
		if err := json.Unmarshal([]byte(row.RequestBusiData), &req); err != nil {
			return nil, err
		}
		return sc.buildFinancePrepared(ctx, row.NetworkCode, req, nonceOverride)
	case "SCPLUS_ISSUE":
		var req models.IssueInvokeRequest
		if err := json.Unmarshal([]byte(row.RequestBusiData), &req); err != nil {
			return nil, err
		}
		return sc.buildIssuePrepared(ctx, row.NetworkCode, req, nonceOverride)
	case "SCPLUS_ISSUE_AND_FINANCE":
		var req models.IssueAndFinanceInvokeRequest
		if err := json.Unmarshal([]byte(row.RequestBusiData), &req); err != nil {
			return nil, err
		}
		return sc.buildIssueFPrepared(ctx, row.NetworkCode, req, nonceOverride)
	default:
		return nil, fmt.Errorf("unsupported onchain type for resign: %s", row.OnchainType)
	}
}

func sanitizeNextNonce(nextNonce uint64) *uint64 {
	if nextNonce == 0 {
		return nil
	}
	return &nextNonce
}

func addDefaultTxSubscription(networkCode, txCode string) {
	if mysql.DB() == nil || txCode == "" {
		return
	}
	record := store.TxSubscriptionPO{
		Code:         fmt.Sprintf("%s_%s", networkCode, txCode),
		TxCode:       txCode,
		NetworkCode:  networkCode,
		EndTimestamp: time.Now().Add(24 * time.Hour).UnixMilli(),
		Status:       "ACTIVE",
	}
	mysql.DB().Where("code = ?", record.Code).Assign(record).FirstOrCreate(&record)
}

func findNetworkConfig(networkCode string) (config.NetworkConfig, error) {
	cfg := config.GetConfig()
	if cfg.Ethereum != nil {
		for _, network := range cfg.Ethereum.Networks {
			if network.Code == networkCode {
				return network, nil
			}
		}
	}
	return config.NetworkConfig{}, errors.New("network config not found")
}

func mustDecodeTx(signedTx string) *types.Transaction {
	raw := common.FromHex(signedTx)
	tx := new(types.Transaction)
	_ = tx.UnmarshalBinary(raw)
	return tx
}

func isAlreadyKnownError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "already known") ||
		strings.Contains(msg, "known transaction") ||
		strings.Contains(msg, "already imported") ||
		strings.Contains(msg, "transaction already exists") ||
		strings.Contains(msg, "was already imported")
}

func parseNonceConflict(err error) (uint64, bool) {
	if err == nil {
		return 0, false
	}
	msg := strings.ToLower(err.Error())
	if strings.Contains(msg, "nonce too low") ||
		strings.Contains(msg, "transaction nonce is too low") ||
		strings.Contains(msg, "replacement transaction underpriced") ||
		strings.Contains(msg, "same nonce already") ||
		strings.Contains(msg, "with the same nonce already imported") ||
		strings.Contains(msg, "same sender and nonce already pending") ||
		strings.Contains(msg, "nonce already used") {
		return 0, true
	}
	re := regexp.MustCompile(`got\s+(\d+).*?expected\s+(\d+)`)
	matches := re.FindStringSubmatch(msg)
	if len(matches) != 3 {
		return 0, false
	}
	got, gotOK := new(big.Int).SetString(matches[1], 10)
	expected, expectedOK := new(big.Int).SetString(matches[2], 10)
	if !gotOK || !expectedOK || got.Cmp(expected) >= 0 {
		return 0, false
	}
	return expected.Uint64(), true
}
