package models

import (
	"encoding/json"
	"time"
)

type TxSendRequest struct {
	TxSignResult  string                 `json:"txSignResult"`
	EntryPoint    string                 `json:"entryPoint"`
	UserOperation map[string]interface{} `json:"userOperation"`
	EIP7702Auth   map[string]interface{} `json:"eip7702Auth"`
}

type TxSendResponse struct {
	TxCode     string `json:"txCode"`
	TxCodeType string `json:"txCodeType"`
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
	Balance     float64 `json:"balance"`
	BalanceUnit string  `json:"balanceUnit"`
}

type LatestBlockResponse struct {
	BlockNumber uint64 `json:"blockNumber"`
	Timestamp   uint64 `json:"timestamp"`
}

type TokenSupplyRequest struct {
	TokenCode string `json:"tokenCode"`
}

type TokenSupplyResponse struct {
	Value float64 `json:"value"`
}

type TokenBalanceRequest struct {
	TokenCode string `json:"tokenCode"`
	Address   string `json:"address"`
}

type TokenBalanceResponse struct {
	Value float64 `json:"value"`
}

type TokenAddRequest struct {
	TokenCode    string `json:"tokenCode"`
	TokenAddress string `json:"tokenAddress"`
	Decimals     int    `json:"decimals"`
}

type TokenGetRequest struct {
	TokenCode string `json:"tokenCode"`
}

type TokenDeleteRequest struct {
	TokenCode string `json:"tokenCode"`
}

type TokenListRequest struct {
}

type TokenInfo struct {
	TokenCode    string `json:"tokenCode"`
	NetworkCode  string `json:"networkCode"`
	TokenAddress string `json:"tokenAddress"`
	Decimals     int    `json:"decimals"`
	CreatedAt    int64  `json:"createdAt"`
	UpdatedAt    int64  `json:"updatedAt"`
}

type TokenListResponse struct {
	Tokens []TokenInfo `json:"tokens"`
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
	Code        string `json:"code"`
	NodeAddress string `json:"nodeAddress"`
	ChainID     int64  `json:"chainId"`
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

func TimeToMillis(t time.Time) int64 {
	if t.IsZero() {
		return 0
	}
	return t.UnixMilli()
}

type ChainTx struct {
	Code           string `json:"code"`
	NetworkCode    string `json:"networkCode"`
	BlockNumber    uint64 `json:"blockNumber"`
	Timestamp      uint64 `json:"timestamp"`
	Fee            string `json:"fee"`
	From           string `json:"from"`
	To             string `json:"to"`
	Amount         string `json:"amount"`
	Status         string `json:"status"`
	SequenceNumber string `json:"sequenceNumber"`
}

type ChainEvent struct {
	Type string      `json:"type"`
	Data interface{} `json:"data"`
}
