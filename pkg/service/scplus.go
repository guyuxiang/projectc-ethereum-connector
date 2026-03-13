package service

import (
	"context"
	"encoding/hex"
	"errors"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/guyuxiang/projectc-ethereum-connector/pkg/models"
)

type ScplusService interface {
	DttSendSettle(ctx context.Context, networkCode string, req models.SettleRequest) (*models.OnchainRecordResponse, error)
	AutoReject(ctx context.Context, networkCode string, req models.AutoRejectRequest) (*models.OnchainRecordResponse, error)
	InstantOnRamp(ctx context.Context, networkCode string, req models.InstantOnRampRequest) (*models.OnchainRecordResponse, error)
	Finance(ctx context.Context, networkCode string, req models.FinanceInvokeRequest) (*models.OnchainRecordResponse, error)
	Issue(ctx context.Context, networkCode string, req models.IssueInvokeRequest) (*models.OnchainRecordResponse, error)
	IssueF(ctx context.Context, networkCode string, req models.IssueAndFinanceInvokeRequest) (*models.OnchainRecordResponse, error)
}

type scplusService struct {
	txSigner *contractTxService
	onchain  OnchainRecordService
}

func NewScplusService(contracts ContractRegistryService, onchain OnchainRecordService) ScplusService {
	return &scplusService{
		txSigner: newContractTxService(contracts),
		onchain:  onchain,
	}
}

const dttTradeABI = `[{"inputs":[{"internalType":"string","name":"businessId","type":"string"}],"name":"settleTrade","outputs":[],"stateMutability":"nonpayable","type":"function"},{"inputs":[{"internalType":"string","name":"scplusId","type":"string"},{"internalType":"address","name":"transferee","type":"address"},{"internalType":"uint8","name":"financeType","type":"uint8"},{"internalType":"address","name":"tokenAddress","type":"address"},{"internalType":"uint256","name":"amount","type":"uint256"},{"internalType":"string","name":"memo","type":"string"},{"internalType":"string","name":"extension","type":"string"}],"name":"financeByAgency","outputs":[],"stateMutability":"nonpayable","type":"function"}]`
const rorMarketABI = `[{"inputs":[{"internalType":"string","name":"transferRefId","type":"string"}],"name":"expire","outputs":[],"stateMutability":"nonpayable","type":"function"}]`
const onRampABI = `[{"inputs":[{"internalType":"address","name":"requester","type":"address"},{"internalType":"address","name":"tokenContract","type":"address"},{"internalType":"uint256","name":"amount","type":"uint256"},{"internalType":"string","name":"businessId","type":"string"},{"internalType":"string","name":"extension","type":"string"}],"name":"instantOnRamp","outputs":[],"stateMutability":"nonpayable","type":"function"}]`
const issueABI = `[{"inputs":[{"internalType":"address","name":"from","type":"address"},{"internalType":"address","name":"to","type":"address"},{"internalType":"address","name":"erc20Address","type":"address"},{"internalType":"uint256","name":"amount","type":"uint256"},{"components":[{"internalType":"string","name":"id","type":"string"},{"internalType":"string","name":"conditionType","type":"string"},{"internalType":"string","name":"description","type":"string"},{"components":[{"internalType":"string","name":"name","type":"string"},{"internalType":"string","name":"value","type":"string"},{"internalType":"bool","name":"changeFlag","type":"bool"},{"internalType":"bool","name":"changeAble","type":"bool"},{"internalType":"address","name":"changeAddr","type":"address"},{"internalType":"uint256","name":"beginTime","type":"uint256"},{"internalType":"uint256","name":"endTime","type":"uint256"},{"internalType":"string","name":"commentsHash","type":"string"},{"internalType":"string[]","name":"filesHash","type":"string[]"}],"internalType":"struct DTTStorage.ConditionFactor[]","name":"fixFactors","type":"tuple[]"},{"components":[{"internalType":"string","name":"name","type":"string"},{"internalType":"string","name":"value","type":"string"},{"internalType":"bool","name":"changeFlag","type":"bool"},{"internalType":"bool","name":"changeAble","type":"bool"},{"internalType":"address","name":"changeAddr","type":"address"},{"internalType":"uint256","name":"beginTime","type":"uint256"},{"internalType":"uint256","name":"endTime","type":"uint256"},{"internalType":"string","name":"commentsHash","type":"string"},{"internalType":"string[]","name":"filesHash","type":"string[]"}],"internalType":"struct DTTStorage.ConditionFactor[]","name":"dynamicFactors","type":"tuple[]"}],"internalType":"struct DTTStorage.SingleCondition[]","name":"scs","type":"tuple[]"},{"components":[{"internalType":"string","name":"id","type":"string"},{"internalType":"string[]","name":"scIDs","type":"string[]"},{"internalType":"string[]","name":"csIDs","type":"string[]"},{"internalType":"enum DTTStorage.JoinType","name":"join","type":"uint8"}],"internalType":"struct DTTStorage.ConditionSet[]","name":"css","type":"tuple[]"},{"internalType":"string","name":"timeScId","type":"string"},{"internalType":"string","name":"csId","type":"string"},{"internalType":"bool","name":"partialAcceptEnable","type":"bool"},{"internalType":"address","name":"partialAcceptAddress","type":"address"},{"internalType":"string","name":"partialAcceptScId","type":"string"},{"internalType":"uint256","name":"guaranteeAmount","type":"uint256"},{"internalType":"string","name":"extension","type":"string"},{"internalType":"string","name":"bid","type":"string"}],"name":"issueByAgency","outputs":[],"stateMutability":"nonpayable","type":"function"},{"inputs":[{"internalType":"address","name":"from","type":"address"},{"internalType":"address","name":"to","type":"address"},{"internalType":"address","name":"erc20Address","type":"address"},{"internalType":"uint256","name":"amount","type":"uint256"},{"components":[{"internalType":"string","name":"id","type":"string"},{"internalType":"string","name":"conditionType","type":"string"},{"internalType":"string","name":"description","type":"string"},{"components":[{"internalType":"string","name":"name","type":"string"},{"internalType":"string","name":"value","type":"string"},{"internalType":"bool","name":"changeFlag","type":"bool"},{"internalType":"bool","name":"changeAble","type":"bool"},{"internalType":"address","name":"changeAddr","type":"address"},{"internalType":"uint256","name":"beginTime","type":"uint256"},{"internalType":"uint256","name":"endTime","type":"uint256"},{"internalType":"string","name":"commentsHash","type":"string"},{"internalType":"string[]","name":"filesHash","type":"string[]"}],"internalType":"struct DTTStorage.ConditionFactor[]","name":"fixFactors","type":"tuple[]"},{"components":[{"internalType":"string","name":"name","type":"string"},{"internalType":"string","name":"value","type":"string"},{"internalType":"bool","name":"changeFlag","type":"bool"},{"internalType":"bool","name":"changeAble","type":"bool"},{"internalType":"address","name":"changeAddr","type":"address"},{"internalType":"uint256","name":"beginTime","type":"uint256"},{"internalType":"uint256","name":"endTime","type":"uint256"},{"internalType":"string","name":"commentsHash","type":"string"},{"internalType":"string[]","name":"filesHash","type":"string[]"}],"internalType":"struct DTTStorage.ConditionFactor[]","name":"dynamicFactors","type":"tuple[]"}],"internalType":"struct DTTStorage.SingleCondition[]","name":"scs","type":"tuple[]"},{"components":[{"internalType":"string","name":"id","type":"string"},{"internalType":"string[]","name":"scIDs","type":"string[]"},{"internalType":"string[]","name":"csIDs","type":"string[]"},{"internalType":"enum DTTStorage.JoinType","name":"join","type":"uint8"}],"internalType":"struct DTTStorage.ConditionSet[]","name":"css","type":"tuple[]"},{"internalType":"string","name":"timeScId","type":"string"},{"internalType":"string","name":"csId","type":"string"},{"internalType":"bool","name":"partialAcceptEnable","type":"bool"},{"internalType":"address","name":"partialAcceptAddress","type":"address"},{"internalType":"string","name":"partialAcceptScId","type":"string"},{"internalType":"uint256","name":"guaranteeAmount","type":"uint256"},{"internalType":"string","name":"issueExtension","type":"string"},{"internalType":"string","name":"bid","type":"string"},{"components":[{"internalType":"address","name":"transferee","type":"address"},{"internalType":"enum RorMarket.ConsiderationType","name":"considerationType","type":"uint8"},{"internalType":"address","name":"considerationDttAddr","type":"address"},{"internalType":"uint256","name":"considerationAmount","type":"uint256"},{"internalType":"string","name":"considerationSelfConfig","type":"string"},{"internalType":"string","name":"financeExtension","type":"string"}],"internalType":"struct IssueFacet.FinanceItems","name":"financeItems","type":"tuple"}],"name":"issueFByAgency","outputs":[],"stateMutability":"nonpayable","type":"function"}]`

type conditionFactor struct {
	Name         string         `abi:"name"`
	Value        string         `abi:"value"`
	ChangeFlag   bool           `abi:"changeFlag"`
	ChangeAble   bool           `abi:"changeAble"`
	ChangeAddr   common.Address `abi:"changeAddr"`
	BeginTime    *big.Int       `abi:"beginTime"`
	EndTime      *big.Int       `abi:"endTime"`
	CommentsHash string         `abi:"commentsHash"`
	FilesHash    []string       `abi:"filesHash"`
}

type singleCondition struct {
	ID             string            `abi:"id"`
	ConditionType  string            `abi:"conditionType"`
	Description    string            `abi:"description"`
	FixFactors     []conditionFactor `abi:"fixFactors"`
	DynamicFactors []conditionFactor `abi:"dynamicFactors"`
}

type conditionSet struct {
	ID    string   `abi:"id"`
	ScIDs []string `abi:"scIDs"`
	CsIDs []string `abi:"csIDs"`
	Join  uint8    `abi:"join"`
}

type financeItems struct {
	Transferee              common.Address `abi:"transferee"`
	ConsiderationType       uint8          `abi:"considerationType"`
	ConsiderationDttAddr    common.Address `abi:"considerationDttAddr"`
	ConsiderationAmount     *big.Int       `abi:"considerationAmount"`
	ConsiderationSelfConfig string         `abi:"considerationSelfConfig"`
	FinanceExtension        string         `abi:"financeExtension"`
}

func (s *scplusService) DttSendSettle(ctx context.Context, networkCode string, req models.SettleRequest) (*models.OnchainRecordResponse, error) {
	prepared, err := s.buildDttSendSettlePrepared(ctx, networkCode, req, nil)
	if err != nil {
		return nil, err
	}
	return s.persistAndSubmit(ctx, prepared)
}

func (s *scplusService) AutoReject(ctx context.Context, networkCode string, req models.AutoRejectRequest) (*models.OnchainRecordResponse, error) {
	prepared, err := s.buildAutoRejectPrepared(ctx, networkCode, req, nil)
	if err != nil {
		return nil, err
	}
	return s.persistAndSubmit(ctx, prepared)
}

func (s *scplusService) InstantOnRamp(ctx context.Context, networkCode string, req models.InstantOnRampRequest) (*models.OnchainRecordResponse, error) {
	prepared, err := s.buildInstantOnRampPrepared(ctx, networkCode, req, nil)
	if err != nil {
		return nil, err
	}
	return s.persistAndSubmit(ctx, prepared)
}

func (s *scplusService) Finance(ctx context.Context, networkCode string, req models.FinanceInvokeRequest) (*models.OnchainRecordResponse, error) {
	prepared, err := s.buildFinancePrepared(ctx, networkCode, req, nil)
	if err != nil {
		return nil, err
	}
	return s.persistAndSubmit(ctx, prepared)
}

func (s *scplusService) Issue(ctx context.Context, networkCode string, req models.IssueInvokeRequest) (*models.OnchainRecordResponse, error) {
	prepared, err := s.buildIssuePrepared(ctx, networkCode, req, nil)
	if err != nil {
		return nil, err
	}
	return s.persistAndSubmit(ctx, prepared)
}

func (s *scplusService) IssueF(ctx context.Context, networkCode string, req models.IssueAndFinanceInvokeRequest) (*models.OnchainRecordResponse, error) {
	prepared, err := s.buildIssueFPrepared(ctx, networkCode, req, nil)
	if err != nil {
		return nil, err
	}
	return s.persistAndSubmit(ctx, prepared)
}

func (s *scplusService) buildDttSendSettlePrepared(ctx context.Context, networkCode string, req models.SettleRequest, nonceOverride *uint64) (*PreparedOnchainRecord, error) {
	return s.txSigner.createPreparedContractTxWithNonce(ctx, networkCode, "SCPLUS_SEND_SETTLE", req.BusinessID, req, req, dttTradeABI, "settleTrade", nonceOverride, req.BusinessID)
}

func (s *scplusService) buildAutoRejectPrepared(ctx context.Context, networkCode string, req models.AutoRejectRequest, nonceOverride *uint64) (*PreparedOnchainRecord, error) {
	return s.txSigner.createPreparedContractTxWithNonce(ctx, networkCode, "SCPLUS_AUTO_REJECT", req.TransferRefID, req, req, rorMarketABI, "expire", nonceOverride, req.TransferRefID)
}

func (s *scplusService) buildInstantOnRampPrepared(ctx context.Context, networkCode string, req models.InstantOnRampRequest, nonceOverride *uint64) (*PreparedOnchainRecord, error) {
	tokenContract, err := s.txSigner.contracts.FindContract(networkCode, req.ContractCode)
	if err != nil {
		return nil, err
	}
	amount, ok := new(big.Int).SetString(req.Value.String(), 10)
	if !ok {
		return nil, errors.New("invalid onramp value")
	}
	return s.txSigner.createPreparedContractTxWithNonce(
		ctx,
		networkCode,
		"SCPLUS_ONRAMP",
		req.BusinessID,
		req,
		req,
		onRampABI,
		"instantOnRamp",
		nonceOverride,
		common.HexToAddress(req.Requester),
		common.HexToAddress(tokenContract.Address),
		amount,
		req.BusinessID,
		req.Extension,
	)
}

func (s *scplusService) buildFinancePrepared(ctx context.Context, networkCode string, req models.FinanceInvokeRequest, nonceOverride *uint64) (*PreparedOnchainRecord, error) {
	transferee := common.HexToAddress(identityToAddress(req.FunderID))
	return s.txSigner.createPreparedContractTxWithNonce(
		ctx,
		networkCode,
		"SCPLUS_FINANCE",
		req.FinanceBusinessID,
		req,
		map[string]interface{}{
			"financeBusinessId": req.FinanceBusinessID,
			"issueBusinessId":   req.IssueBusinessID,
			"funderAddress":     transferee.Hex(),
			"extension":         req.Extension,
		},
		dttTradeABI,
		"financeByAgency",
		nonceOverride,
		"SCPLUS"+req.IssueBusinessID,
		transferee,
		uint8(0),
		common.Address{},
		big.NewInt(0),
		"",
		req.Extension,
	)
}

func (s *scplusService) buildIssuePrepared(ctx context.Context, networkCode string, req models.IssueInvokeRequest, nonceOverride *uint64) (*PreparedOnchainRecord, error) {
	signerCfg, _, err := findOnchainSignerConfig(networkCode, "SCPLUS_ISSUE")
	if err != nil {
		return nil, err
	}
	amount, ok := new(big.Int).SetString(req.Amount.String(), 10)
	if !ok {
		return nil, errors.New("invalid issue amount")
	}
	from := common.HexToAddress(identityToAddress(req.ObligorID))
	to := common.HexToAddress(identityToAddress(req.BeneficiaryID))
	return s.txSigner.createPreparedContractTxWithNonce(
		ctx,
		networkCode,
		"SCPLUS_ISSUE",
		req.BusinessID,
		req,
		map[string]interface{}{
			"businessId":    req.BusinessID,
			"bid":           "SCPLUS" + req.BusinessID,
			"from":          from.Hex(),
			"to":            to.Hex(),
			"agencyAddress": deriveSignerAddress(signerCfg.PrivateKey),
			"extension":     req.Extension,
		},
		issueABI,
		"issueByAgency",
		nonceOverride,
		from,
		to,
		common.HexToAddress(signerCfg.DefaultTokenAddress),
		amount,
		defaultSingleConditions(req.DueTime),
		[]conditionSet{},
		"SC0",
		"",
		false,
		common.Address{},
		"",
		big.NewInt(0),
		req.Extension,
		"SCPLUS"+req.BusinessID,
	)
}

func (s *scplusService) buildIssueFPrepared(ctx context.Context, networkCode string, req models.IssueAndFinanceInvokeRequest, nonceOverride *uint64) (*PreparedOnchainRecord, error) {
	signerCfg, _, err := findOnchainSignerConfig(networkCode, "SCPLUS_ISSUE_AND_FINANCE")
	if err != nil {
		return nil, err
	}
	amount, ok := new(big.Int).SetString(req.Amount.String(), 10)
	if !ok {
		return nil, errors.New("invalid issueF amount")
	}
	from := common.HexToAddress(identityToAddress(req.ObligorID))
	to := common.HexToAddress(identityToAddress(req.BeneficiaryID))
	transferee := common.HexToAddress(identityToAddress(req.FunderID))
	return s.txSigner.createPreparedContractTxWithNonce(
		ctx,
		networkCode,
		"SCPLUS_ISSUE_AND_FINANCE",
		req.BusinessID,
		req,
		map[string]interface{}{
			"businessId":    req.BusinessID,
			"bid":           "SCPLUS" + req.BusinessID,
			"from":          from.Hex(),
			"to":            to.Hex(),
			"agencyAddress": deriveSignerAddress(signerCfg.PrivateKey),
			"transferee":    transferee.Hex(),
			"extension":     req.Extension,
		},
		issueABI,
		"issueFByAgency",
		nonceOverride,
		from,
		to,
		common.HexToAddress(signerCfg.DefaultTokenAddress),
		amount,
		defaultSingleConditions(req.DueTime),
		[]conditionSet{},
		"SC0",
		"",
		false,
		common.Address{},
		"",
		big.NewInt(0),
		req.Extension,
		"SCPLUS"+req.BusinessID,
		financeItems{
			Transferee:              transferee,
			ConsiderationType:       0,
			ConsiderationDttAddr:    common.Address{},
			ConsiderationAmount:     big.NewInt(0),
			ConsiderationSelfConfig: "",
			FinanceExtension:        "",
		},
	)
}

func (s *scplusService) persistAndSubmit(ctx context.Context, prepared *PreparedOnchainRecord) (*models.OnchainRecordResponse, error) {
	if _, err := s.onchain.CreatePrepared(*prepared); err != nil {
		return nil, err
	}
	if submitErr := s.onchain.Submit(ctx, prepared.Code); submitErr != nil {
		return nil, submitErr
	}
	return s.onchain.Get(prepared.OnchainType, prepared.IdempotencyKey)
}

func identityToAddress(identity string) string {
	sum := crypto.Keccak256([]byte(identity))
	return "0x" + hex.EncodeToString(sum[len(sum)-20:])
}

func defaultSingleConditions(dueTime int64) []singleCondition {
	return []singleCondition{
		{
			ID:            "SC0",
			ConditionType: "T4",
			Description:   "Only on or after the date [Date]",
			FixFactors: []conditionFactor{
				{
					Name:         "DATE",
					Value:        big.NewInt(dueTime).String(),
					ChangeFlag:   false,
					ChangeAble:   false,
					ChangeAddr:   common.Address{},
					BeginTime:    big.NewInt(0),
					EndTime:      big.NewInt(0),
					CommentsHash: "",
					FilesHash:    []string{},
				},
			},
			DynamicFactors: []conditionFactor{},
		},
	}
}

func deriveSignerAddress(privateKey string) string {
	key, err := crypto.HexToECDSA(trimHexPrefix(privateKey))
	if err != nil {
		return ""
	}
	return crypto.PubkeyToAddress(key.PublicKey).Hex()
}

func trimHexPrefix(value string) string {
	return strings.TrimPrefix(value, "0x")
}
