package models

import "encoding/json"

type TxSendRequest struct {
	TxSignResult string `json:"txSignResult"`
}

type TxSendResponse struct {
	TxCode string `json:"txCode"`
}

type TxQueryRequest struct {
	TxCode string `json:"txCode"`
}

type TxQueryResponse struct {
	IfTxOnchain bool         `json:"ifTxOnchain"`
	Tx          *ChainTx     `json:"tx"`
	TxEvents    []ChainEvent `json:"txEvents"`
}

type AddressBalanceRequest struct {
	Address string `json:"address"`
}

type AddressBalanceResponse struct {
	Balance     RawNumber `json:"balance"`
	BalanceUnit string    `json:"balanceUnit"`
}

type LatestBlockResponse struct {
	BlockNumber uint64 `json:"blockNumber"`
	Timestamp   uint64 `json:"timestamp"`
}

type TokenSupplyRequest struct {
	TokenCode string `json:"tokenCode"`
}

type TokenSupplyResponse struct {
	Value RawNumber `json:"value"`
}

type TokenBalanceRequest struct {
	TokenCode string `json:"tokenCode"`
	Address   string `json:"address"`
}

type TokenBalanceResponse struct {
	Value RawNumber `json:"value"`
}

type BalanceChargeRequest struct {
	ReceiverAddress string      `json:"receiverAddress"`
	IdempotencyKey  string      `json:"idempotencyKey"`
	Value           json.Number `json:"value"`
}

type OnchainRecordResponse struct {
	IdempotencyKey   string   `json:"idempotencyKey"`
	OnchainStatus    string   `json:"onchainStatus"`
	ResponseBusiData string   `json:"responseBusiData"`
	ChainTxData      *ChainTx `json:"chainTxData"`
	TxCode           string   `json:"txCode"`
}

type SettleRequest struct {
	BusinessID string `json:"businessId"`
}

type AutoRejectRequest struct {
	TransferRefID string `json:"transferRefId"`
}

type InstantOnRampRequest struct {
	BusinessID   string      `json:"businessId"`
	ContractCode string      `json:"contractCode"`
	Requester    string      `json:"requester"`
	Value        json.Number `json:"value"`
	Extension    string      `json:"extension"`
}

type IssueInvokeRequest struct {
	ObligorID     string      `json:"obligorId"`
	BeneficiaryID string      `json:"beneficiaryId"`
	Amount        json.Number `json:"amount"`
	DueTime       int64       `json:"dueTime"`
	Extension     string      `json:"extension"`
	BusinessID    string      `json:"businessId"`
}

type BusinessQueryRequest struct {
	BusinessID string `json:"businessId"`
}

type FinanceInvokeRequest struct {
	FinanceBusinessID string `json:"financeBusinessId"`
	IssueBusinessID   string `json:"issueBusinessId"`
	FunderID          string `json:"funderId"`
	Extension         string `json:"extension"`
}

type IssueAndFinanceInvokeRequest struct {
	ObligorID     string      `json:"obligorId"`
	BeneficiaryID string      `json:"beneficiaryId"`
	FunderID      string      `json:"funderId"`
	Amount        json.Number `json:"amount"`
	DueTime       int64       `json:"dueTime"`
	Extension     string      `json:"extension"`
	BusinessID    string      `json:"businessId"`
}

type TxSubscribeRequest struct {
	TxCode         string           `json:"txCode"`
	SubscribeRange TxSubscribeRange `json:"subscribeRange"`
}

type TxSubscribeRange struct {
	EndTimestamp int64 `json:"endTimestamp"`
}

type AddressSubscribeRequest struct {
	Address        string                `json:"address"`
	SubscribeRange AddressSubscribeRange `json:"subscribeRange"`
}

type AddressSubscribeRange struct {
	StartBlockNumber uint64 `json:"startBlockNumber"`
	EndBlockNumber   uint64 `json:"endBlockNumber"`
}

type TxSubscribeCancelRequest struct {
	TxCode string `json:"txCode"`
}

type AddressSubscribeCancelRequest struct {
	Address        string `json:"address"`
	EndBlockNumber uint64 `json:"endBlockNumber"`
}

type ContractInfo struct {
	Code                string `json:"code"`
	NetworkCode         string `json:"networkCode"`
	Address             string `json:"address"`
	InterfaceDefinition string `json:"interfaceDefinition"`
}

type ContractListResponse struct {
	ContractInfos []ContractInfo `json:"contractInfos"`
}

type ContractConfigPushMessage struct {
	PushID      string                   `json:"pushId"`
	Description string                   `json:"description"`
	PushItems   []ContractConfigPushItem `json:"pushItems"`
}

type ContractConfigPushItem struct {
	NetworkCode           string `json:"networkCode"`
	ContractCode          string `json:"contractCode"`
	ContractAddress       string `json:"contractAddress"`
	ContractABI           string `json:"contractAbi"`
	ContractDeployTxBlock uint64 `json:"contractDeployTxBlockNumber"`
}

type ApplyContractConfigPushRecordRequest struct {
	PushRecordCode string `json:"pushRecordCode"`
}

type ContractConfigPushRecordQuery struct {
	CodeContains        string `json:"codeContains"`
	DescriptionContains string `json:"descriptionContains"`
}

type ContractConfigPushRecordDTO struct {
	Code        string `json:"code"`
	Description string `json:"description"`
}

type Web3Network struct {
	Code                  string `json:"code"`
	NodeAddress           string `json:"nodeAddress"`
	ChainID               int64  `json:"chainId"`
	BlockchainExplorerURL string `json:"blockchainExplorerUrl"`
}

type Web3Contract struct {
	Code        string `json:"code"`
	NetworkCode string `json:"networkCode"`
	Address     string `json:"address"`
	ABI         string `json:"abi"`
}

type Web3ContractInfo struct {
	Contract Web3Contract `json:"contract"`
	Network  Web3Network  `json:"network"`
}

type Web3ContractInfoResponse struct {
	Web3ContractInfos []Web3ContractInfo `json:"web3ContractInfos"`
}

type ChainTx struct {
	Code           string    `json:"code"`
	NetworkCode    string    `json:"networkCode"`
	BlockNumber    uint64    `json:"blockNumber"`
	Timestamp      uint64    `json:"timestamp"`
	Fee            string    `json:"fee"`
	From           string    `json:"from"`
	To             string    `json:"to"`
	Amount         RawNumber `json:"amount"`
	Status         string    `json:"status"`
	SequenceNumber string    `json:"sequenceNumber"`
}

type ChainEvent struct {
	Code        string `json:"code"`
	NetworkCode string `json:"networkCode"`
	BlockNumber uint64 `json:"blockNumber"`
	Timestamp   uint64 `json:"timestamp"`
	Fee         string `json:"fee"`
	Type        string `json:"type"`
	DataType    string `json:"dataType"`
	LogIndex    string `json:"logIndex"`
	Data        string `json:"data"`
}
