package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"regexp"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/guyuxiang/projectc-ethereum-connector/pkg/models"
	"github.com/guyuxiang/projectc-ethereum-connector/pkg/mysql"
	"github.com/guyuxiang/projectc-ethereum-connector/pkg/store"
)

type OnchainRecordService interface {
	Create(kind, key string, payload interface{}) *models.OnchainRecordResponse
	CreatePrepared(record PreparedOnchainRecord) (*models.OnchainRecordResponse, error)
	Get(kind, key string) (*models.OnchainRecordResponse, error)
	Submit(ctx context.Context, code string) error
	Refresh(ctx context.Context) error
}

type onchainRecordService struct{}

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
	return &onchainRecordService{}
}

func (s *onchainRecordService) CreatePrepared(record PreparedOnchainRecord) (*models.OnchainRecordResponse, error) {
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
	record, err := s.createInDB(kind, key, payload)
	if err != nil {
		responseData, _ := json.Marshal(payload)
		return &models.OnchainRecordResponse{
			IdempotencyKey:   key,
			OnchainStatus:    "FAILED",
			ResponseBusiData: string(responseData),
			TxCode:           "",
		}
	}
	return record
}

func (s *onchainRecordService) Get(kind, key string) (*models.OnchainRecordResponse, error) {
	return s.getFromDB(kind, key)
}

func (s *onchainRecordService) Submit(ctx context.Context, code string) error {
	var row store.OnchainRecordPO
	if err := mysql.DB().Where("code = ?", code).First(&row).Error; err != nil {
		return err
	}
	if row.OnchainStatus != "INIT" && row.OnchainStatus != "TO_BE_RESIGN" {
		return nil
	}

	network, err := configuredNetwork()
	if err != nil {
		return err
	}
	client, err := ethclient.DialContext(ctx, network.Rpcurl)
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
		network, err := configuredNetwork()
		if err != nil {
			continue
		}
		client, err := ethclient.DialContext(ctx, network.Rpcurl)
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
	switch row.OnchainType {
	case "NATIVE_TOKEN_CHARGE":
		var req models.BalanceChargeRequest
		if err := json.Unmarshal([]byte(row.RequestBusiData), &req); err != nil {
			return nil, err
		}
		wallet := &walletService{nonce: NewSignerNonceService(), onchain: s}
		return wallet.createNativeChargePrepared(ctx, req, nonceOverride)
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
	if txCode == "" {
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
