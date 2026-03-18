package service

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/guyuxiang/projectc-ethereum-connector/pkg/config"
	"github.com/guyuxiang/projectc-ethereum-connector/pkg/models"
)

type EthereumService interface {
	SendTransaction(ctx context.Context, req models.TxSendRequest) (*models.TxSendResponse, error)
	QueryTransaction(ctx context.Context, txHash string) (*models.TxQueryResponse, error)
	QueryLogs(ctx context.Context, address string, fromBlock, toBlock uint64) ([]rpcLogRecord, error)
	DecodeLogEvent(logEntry rpcLogRecord) (string, string, string)
	GetAddressBalance(ctx context.Context, address string) (*models.AddressBalanceResponse, error)
	GetLatestBlock(ctx context.Context) (*models.LatestBlockResponse, error)
	GetTokenSupply(ctx context.Context, tokenCode string) (*models.TokenSupplyResponse, error)
	GetTokenBalance(ctx context.Context, tokenCode, address string) (*models.TokenBalanceResponse, error)
}

type ethereumService struct {
	client    *http.Client
	network   config.NetworkConfig
	contracts ContractRegistryService
	tokens    TokenRegistryService
	abiMu     sync.RWMutex
	abiCache  map[string]cachedABI
}

type cachedABI struct {
	definition string
	parsed     abi.ABI
}

func NewEthereumService(contracts ContractRegistryService, tokens TokenRegistryService) EthereumService {
	network, _ := configuredNetwork()
	return &ethereumService{
		client:    &http.Client{Timeout: 15 * time.Second},
		network:   network,
		contracts: contracts,
		tokens:    tokens,
		abiCache:  map[string]cachedABI{},
	}
}

func (s *ethereumService) SendTransaction(ctx context.Context, req models.TxSendRequest) (*models.TxSendResponse, error) {
	if strings.TrimSpace(req.TxSignResult) != "" && len(req.UserOperation) > 0 {
		return nil, errors.New("txSignResult and userOperation cannot be provided together")
	}

	switch {
	case strings.TrimSpace(req.TxSignResult) != "":
		txHash, err := s.sendRawTransaction(ctx, req.TxSignResult)
		if err != nil {
			return nil, err
		}
		return &models.TxSendResponse{
			TxCode:     txHash,
			TxCodeType: "txHash",
		}, nil
	case len(req.UserOperation) > 0:
		userOpHash, err := s.sendUserOperation(ctx, req.UserOperation, req.EntryPoint, req.EIP7702Auth)
		if err != nil {
			return nil, err
		}
		return &models.TxSendResponse{
			TxCode:     userOpHash,
			TxCodeType: "userOpHash",
		}, nil
	default:
		return nil, errors.New("either txSignResult or userOperation is required")
	}
}

func (s *ethereumService) sendRawTransaction(ctx context.Context, signedTx string) (string, error) {
	signedTx = strings.TrimSpace(signedTx)
	if signedTx == "" {
		return "", errors.New("txSignResult is required for rawTransaction")
	}

	var txHash string
	if err := s.rpcCall(ctx, "eth_sendRawTransaction", []interface{}{signedTx}, &txHash); err != nil {
		return "", err
	}
	return txHash, nil
}

func (s *ethereumService) sendUserOperation(ctx context.Context, userOperation map[string]interface{}, entryPoint string, eip7702Auth map[string]interface{}) (string, error) {
	if len(userOperation) == 0 {
		return "", errors.New("userOperation is required")
	}
	entryPoint = strings.TrimSpace(entryPoint)
	if entryPoint == "" {
		return "", errors.New("entryPoint is required")
	}

	params := []interface{}{userOperation, entryPoint}
	if len(eip7702Auth) > 0 {
		params = append(params, eip7702Auth)
	}

	var userOpHash string
	if err := s.rpcCallToURL(ctx, s.bundlerURL(), "eth_sendUserOperation", params, &userOpHash); err != nil {
		return "", err
	}
	return userOpHash, nil
}

func (s *ethereumService) QueryTransaction(ctx context.Context, txHash string) (*models.TxQueryResponse, error) {
	resp, err := s.queryNativeTransaction(ctx, txHash)
	if err == nil {
		return resp, nil
	}
	if !errors.Is(err, errRPCNullResult) {
		return nil, err
	}
	return s.queryUserOperation(ctx, txHash)
}

func (s *ethereumService) queryNativeTransaction(ctx context.Context, txHash string) (*models.TxQueryResponse, error) {
	networkCode := s.network.Networkcode
	var tx rpcTransaction
	if err := s.rpcCall(ctx, "eth_getTransactionByHash", []interface{}{txHash}, &tx); err != nil {
		return nil, err
	}

	var receipt rpcReceipt
	if err := s.rpcCall(ctx, "eth_getTransactionReceipt", []interface{}{txHash}, &receipt); err != nil {
		if !errors.Is(err, errRPCNullResult) {
			return nil, err
		}
	}

	blockTimestamp := uint64(0)
	if receipt.BlockNumber != "" {
		block, err := s.getBlockByNumber(ctx, receipt.BlockNumber)
		if err == nil {
			blockTimestamp = hexUint64(block.Timestamp) * 1000
		}
	}

	status := "FAILED"
	if receipt.Status == "" || receipt.Status == "0x1" {
		status = "SUCCESS"
	}

	result := &models.TxQueryResponse{
		IfTxOnchain: true,
		Tx: &models.ChainTx{
			Code:           tx.Hash,
			NetworkCode:    networkCode,
			BlockNumber:    hexUint64(receipt.BlockNumber),
			Timestamp:      blockTimestamp,
			Fee:            calcFee(receipt.EffectiveGasPrice, receipt.GasUsed),
			From:           tx.From,
			To:             tx.To,
			Amount:         hexToBigIntString(tx.Value),
			Status:         status,
			SequenceNumber: fmt.Sprintf("%d", hexUint64(tx.Nonce)),
		},
		TxEvents: make([]models.ChainEvent, 0, len(receipt.Logs)),
	}

	for _, logEntry := range receipt.Logs {
		eventType, _, eventData := s.decodeLogEvent(networkCode, logEntry)
		if strings.TrimSpace(eventType) == "" {
			continue
		}
		result.TxEvents = append(result.TxEvents, models.ChainEvent{
			Type: eventType,
			Data: decodeEventDataValue(eventData),
		})
	}

	return result, nil
}

func (s *ethereumService) queryUserOperation(ctx context.Context, userOpHash string) (*models.TxQueryResponse, error) {
	var receipt rpcUserOperationReceipt
	if err := s.rpcCallToURL(ctx, s.bundlerURL(), "eth_getUserOperationReceipt", []interface{}{userOpHash}, &receipt); err != nil {
		if errors.Is(err, errRPCNullResult) {
			var pending rpcUserOperationByHash
			if pendingErr := s.rpcCallToURL(ctx, s.bundlerURL(), "eth_getUserOperationByHash", []interface{}{userOpHash}, &pending); pendingErr != nil {
				if errors.Is(pendingErr, errRPCNullResult) {
					return &models.TxQueryResponse{IfTxOnchain: false}, nil
				}
				return nil, pendingErr
			}
			return &models.TxQueryResponse{IfTxOnchain: false}, nil
		}
		return nil, err
	}

	blockTimestamp := uint64(0)
	if receipt.Receipt.BlockNumber != "" {
		block, err := s.getBlockByNumber(ctx, receipt.Receipt.BlockNumber)
		if err == nil {
			blockTimestamp = hexUint64(block.Timestamp) * 1000
		}
	}

	status := "FAILED"
	if receipt.Success || receipt.Receipt.Status == "" || receipt.Receipt.Status == "0x1" {
		status = "SUCCESS"
	}
	fee := calcFee(receipt.Receipt.EffectiveGasPrice, receipt.Receipt.GasUsed)
	if fee == "0" && receipt.ActualGasCost != "" {
		fee = hexToBigIntString(receipt.ActualGasCost)
	}

	result := &models.TxQueryResponse{
		IfTxOnchain: true,
		Tx: &models.ChainTx{
			Code:           userOpHash,
			NetworkCode:    s.network.Networkcode,
			BlockNumber:    hexUint64(receipt.Receipt.BlockNumber),
			Timestamp:      blockTimestamp,
			Fee:            fee,
			From:           receipt.Sender,
			To:             receipt.Receipt.To,
			Amount:         "0",
			Status:         status,
			SequenceNumber: fmt.Sprintf("%d", hexUint64(receipt.Nonce)),
		},
		TxEvents: make([]models.ChainEvent, 0, len(receipt.Logs)),
	}

	for _, logEntry := range receipt.Logs {
		eventType, _, eventData := s.decodeLogEvent(s.network.Networkcode, logEntry)
		if strings.TrimSpace(eventType) == "" {
			continue
		}
		result.TxEvents = append(result.TxEvents, models.ChainEvent{
			Type: eventType,
			Data: decodeEventDataValue(eventData),
		})
	}

	return result, nil
}

func (s *ethereumService) GetAddressBalance(ctx context.Context, address string) (*models.AddressBalanceResponse, error) {
	var balanceHex string
	if err := s.rpcCall(ctx, "eth_getBalance", []interface{}{address, "latest"}, &balanceHex); err != nil {
		return nil, err
	}

	balance, ok := new(big.Int).SetString(strings.TrimPrefix(balanceHex, "0x"), 16)
	if !ok {
		return nil, fmt.Errorf("invalid balance hex: %s", balanceHex)
	}

	return &models.AddressBalanceResponse{
		Balance:     formatBigIntFloat64(balance, 18),
		BalanceUnit: s.nativeBalanceUnit(),
	}, nil
}

func (s *ethereumService) GetLatestBlock(ctx context.Context) (*models.LatestBlockResponse, error) {
	block, err := s.getBlockByNumber(ctx, "latest")
	if err != nil {
		return nil, err
	}
	return &models.LatestBlockResponse{
		BlockNumber: hexUint64(block.Number),
		Timestamp:   hexUint64(block.Timestamp) * 1000,
	}, nil
}

func (s *ethereumService) nativeBalanceUnit() string {
	symbol := strings.TrimSpace(s.network.Nativesymbol)
	if symbol == "" {
		return "ETH"
	}
	return symbol
}

func (s *ethereumService) GetTokenSupply(ctx context.Context, tokenCode string) (*models.TokenSupplyResponse, error) {
	token, err := s.tokens.FindToken(tokenCode)
	if err != nil {
		return nil, err
	}
	value, err := s.ethCall(ctx, token.TokenAddress, "0x18160ddd")
	if err != nil {
		return nil, err
	}
	return &models.TokenSupplyResponse{Value: formatTokenAmountFloat64(value, token.Decimals)}, nil
}

func (s *ethereumService) GetTokenBalance(ctx context.Context, tokenCode, address string) (*models.TokenBalanceResponse, error) {
	token, err := s.tokens.FindToken(tokenCode)
	if err != nil {
		return nil, err
	}

	paddedAddress := leftPadHex(strings.TrimPrefix(strings.ToLower(address), "0x"), 64)
	value, err := s.ethCall(ctx, token.TokenAddress, "0x70a08231"+paddedAddress)
	if err != nil {
		return nil, err
	}
	return &models.TokenBalanceResponse{Value: formatTokenAmountFloat64(value, token.Decimals)}, nil
}

func (s *ethereumService) QueryLogs(ctx context.Context, address string, fromBlock, toBlock uint64) ([]rpcLogRecord, error) {
	var logs []rpcLogRecord
	if err := s.rpcCall(ctx, "eth_getLogs", []interface{}{
		map[string]interface{}{
			"address":   address,
			"fromBlock": fmt.Sprintf("0x%x", fromBlock),
			"toBlock":   fmt.Sprintf("0x%x", toBlock),
		},
	}, &logs); err != nil {
		return nil, err
	}
	return logs, nil
}

func (s *ethereumService) DecodeLogEvent(logEntry rpcLogRecord) (string, string, string) {
	return s.decodeLogEvent(s.network.Networkcode, logEntry)
}

func (s *ethereumService) getBlockByNumber(ctx context.Context, blockNumber string) (*rpcBlock, error) {
	var block rpcBlock
	if err := s.rpcCall(ctx, "eth_getBlockByNumber", []interface{}{blockNumber, false}, &block); err != nil {
		return nil, err
	}
	return &block, nil
}

func (s *ethereumService) ethCall(ctx context.Context, to, data string) (string, error) {
	var result string
	if err := s.rpcCall(ctx, "eth_call", []interface{}{
		map[string]interface{}{"to": to, "data": data},
		"latest",
	}, &result); err != nil {
		return "", err
	}
	return result, nil
}

func (s *ethereumService) bundlerURL() string {
	if strings.TrimSpace(s.network.Bundlerurl) != "" {
		return s.network.Bundlerurl
	}
	return s.network.Rpcurl
}

var errRPCNullResult = errors.New("rpc returned null result")

func (s *ethereumService) rpcCall(ctx context.Context, method string, params []interface{}, out interface{}) error {
	return s.rpcCallToURL(ctx, s.network.Rpcurl, method, params, out)
}

func (s *ethereumService) rpcCallToURL(ctx context.Context, rpcURL, method string, params []interface{}, out interface{}) error {
	if strings.TrimSpace(rpcURL) == "" {
		return fmt.Errorf("ethereum network is not configured")
	}

	body, err := json.Marshal(map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  method,
		"params":  params,
	})
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, rpcURL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	payload, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	var rpcResp rpcResponse
	if err = json.Unmarshal(payload, &rpcResp); err != nil {
		return err
	}
	if rpcResp.Error != nil {
		return fmt.Errorf("rpc error %d: %s", rpcResp.Error.Code, rpcResp.Error.Message)
	}
	if string(rpcResp.Result) == "null" {
		return errRPCNullResult
	}
	if out == nil {
		return nil
	}
	return json.Unmarshal(rpcResp.Result, out)
}

type rpcResponse struct {
	Result json.RawMessage `json:"result"`
	Error  *rpcError       `json:"error"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type rpcTransaction struct {
	Hash  string `json:"hash"`
	From  string `json:"from"`
	To    string `json:"to"`
	Value string `json:"value"`
	Nonce string `json:"nonce"`
}

type rpcReceipt struct {
	BlockNumber       string         `json:"blockNumber"`
	Status            string         `json:"status"`
	From              string         `json:"from"`
	To                string         `json:"to"`
	TransactionHash   string         `json:"transactionHash"`
	GasUsed           string         `json:"gasUsed"`
	EffectiveGasPrice string         `json:"effectiveGasPrice"`
	Logs              []rpcLogRecord `json:"logs"`
}

type rpcUserOperationReceipt struct {
	UserOpHash    string         `json:"userOpHash"`
	EntryPoint    string         `json:"entryPoint"`
	Sender        string         `json:"sender"`
	Nonce         string         `json:"nonce"`
	Paymaster     string         `json:"paymaster"`
	ActualGasCost string         `json:"actualGasCost"`
	ActualGasUsed string         `json:"actualGasUsed"`
	Success       bool           `json:"success"`
	Reason        string         `json:"reason"`
	Logs          []rpcLogRecord `json:"logs"`
	Receipt       rpcReceipt     `json:"receipt"`
}

type rpcUserOperationByHash struct {
	UserOperation   map[string]interface{} `json:"userOperation"`
	EntryPoint      string                 `json:"entryPoint"`
	BlockNumber     string                 `json:"blockNumber"`
	BlockHash       string                 `json:"blockHash"`
	TransactionHash string                 `json:"transactionHash"`
}

type rpcLogRecord struct {
	BlockNumber string   `json:"blockNumber"`
	TxHash      string   `json:"transactionHash"`
	Address     string   `json:"address"`
	LogIndex    string   `json:"logIndex"`
	Topics      []string `json:"topics"`
	Data        string   `json:"data"`
}

type rpcBlock struct {
	Number    string `json:"number"`
	Timestamp string `json:"timestamp"`
}

func formatWeiToEther(wei *big.Int) string {
	if wei == nil {
		return "0"
	}
	rat := new(big.Rat).SetInt(wei)
	denominator := new(big.Rat).SetInt(new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil))
	return new(big.Rat).Quo(rat, denominator).FloatString(18)
}

func calcFee(priceHex, gasUsedHex string) string {
	price := hexToBigInt(priceHex)
	gasUsed := hexToBigInt(gasUsedHex)
	if price == nil || gasUsed == nil {
		return "0"
	}
	return new(big.Int).Mul(price, gasUsed).String()
}

func hexUint64(value string) uint64 {
	if value == "" {
		return 0
	}
	out, _ := new(big.Int).SetString(strings.TrimPrefix(value, "0x"), 16)
	if out == nil {
		return 0
	}
	return out.Uint64()
}

func hexToBigIntString(value string) string {
	number := hexToBigInt(value)
	if number == nil {
		return "0"
	}
	return number.String()
}

func formatTokenAmountFloat64(value string, decimals int) float64 {
	number := hexToBigInt(value)
	if number == nil {
		return 0
	}
	return formatBigIntFloat64(number, decimals)
}

func formatBigIntFloat64(number *big.Int, decimals int) float64 {
	if number == nil {
		return 0
	}
	if decimals <= 0 {
		result, _ := new(big.Float).SetInt(number).Float64()
		return result
	}
	base := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(decimals)), nil)
	result, _ := new(big.Float).Quo(new(big.Float).SetInt(number), new(big.Float).SetInt(base)).Float64()
	return result
}

func hexToBigInt(value string) *big.Int {
	if value == "" {
		return big.NewInt(0)
	}
	trimmed := strings.TrimPrefix(value, "0x")
	if trimmed == "" {
		return big.NewInt(0)
	}
	if len(trimmed)%2 == 1 {
		trimmed = "0" + trimmed
	}
	raw, err := hex.DecodeString(trimmed)
	if err != nil {
		return nil
	}
	return new(big.Int).SetBytes(raw)
}

func leftPadHex(value string, width int) string {
	if len(value) >= width {
		return value[len(value)-width:]
	}
	return strings.Repeat("0", width-len(value)) + value
}

func decodeEventDataValue(raw string) interface{} {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}
	var out interface{}
	if err := json.Unmarshal([]byte(trimmed), &out); err == nil {
		return out
	}
	return raw
}

func (s *ethereumService) decodeLogEvent(networkCode string, logEntry rpcLogRecord) (string, string, string) {
	contract := s.findContractByAddress(logEntry.Address)
	if contract == nil || contract.InterfaceDefinition == "" || len(logEntry.Topics) == 0 {
		return "", "", logEntry.Data
	}
	parsedABI, err := s.parsedABI(contract)
	if err != nil {
		return "", "", logEntry.Data
	}

	topic0 := common.HexToHash(logEntry.Topics[0])
	var matched *abi.Event
	for _, event := range parsedABI.Events {
		if event.ID == topic0 {
			copyEvent := event
			matched = &copyEvent
			break
		}
	}
	if matched == nil {
		return "", "", logEntry.Data
	}

	payload := map[string]interface{}{
		"contractCode": contract.Code,
		"contractAddr": contract.Address,
	}
	if unpacked, unpackErr := matched.Inputs.NonIndexed().Unpack(common.FromHex(logEntry.Data)); unpackErr == nil {
		nonIndexedPos := 0
		for _, input := range matched.Inputs {
			if input.Indexed {
				continue
			}
			if nonIndexedPos < len(unpacked) {
				payload[input.Name] = normalizeABIValue(unpacked[nonIndexedPos])
			}
			nonIndexedPos++
		}
	}
	indexedArgs := make(abi.Arguments, 0, len(matched.Inputs))
	for _, input := range matched.Inputs {
		if input.Indexed {
			indexedArgs = append(indexedArgs, input)
		}
	}
	if len(indexedArgs) > 0 && len(logEntry.Topics) > 1 {
		topics := make([]common.Hash, 0, len(logEntry.Topics)-1)
		for _, topic := range logEntry.Topics[1:] {
			topics = append(topics, common.HexToHash(topic))
		}
		topicMap := map[string]interface{}{}
		if parseErr := abi.ParseTopicsIntoMap(topicMap, indexedArgs, topics); parseErr == nil {
			for key, value := range topicMap {
				payload[key] = normalizeABIValue(value)
			}
		}
	}

	raw, err := json.Marshal(payload)
	if err != nil {
		return determineChainEventType(matched.Name, contract.Code), matched.Name, logEntry.Data
	}
	eventType := determineChainEventType(matched.Name, contract.Code)
	return eventType, matched.Name, s.transformEventData(eventType, contract, string(raw))
}

func (s *ethereumService) findContractByAddress(address string) *models.ContractInfo {
	returnContract, err := s.contracts.FindContractByAddress(address)
	if err != nil {
		return nil
	}
	return returnContract
}

func (s *ethereumService) parsedABI(contract *models.ContractInfo) (abi.ABI, error) {
	key := strings.ToLower(contract.Address)
	s.abiMu.RLock()
	cached, ok := s.abiCache[key]
	s.abiMu.RUnlock()
	if ok && cached.definition == contract.InterfaceDefinition {
		return cached.parsed, nil
	}
	parsed, err := abi.JSON(strings.NewReader(contract.InterfaceDefinition))
	if err != nil {
		return abi.ABI{}, err
	}
	s.abiMu.Lock()
	s.abiCache[key] = cachedABI{
		definition: contract.InterfaceDefinition,
		parsed:     parsed,
	}
	s.abiMu.Unlock()
	return parsed, nil
}

func normalizeABIValue(value interface{}) interface{} {
	if value == nil {
		return nil
	}
	switch v := value.(type) {
	case common.Address:
		return strings.ToLower(v.Hex())
	case *big.Int:
		if v == nil {
			return "0"
		}
		return v.String()
	case big.Int:
		return v.String()
	case []byte:
		return "0x" + hex.EncodeToString(v)
	case [32]byte:
		return "0x" + hex.EncodeToString(v[:])
	}
	rv := reflect.ValueOf(value)
	for rv.Kind() == reflect.Pointer {
		if rv.IsNil() {
			return nil
		}
		rv = rv.Elem()
	}
	switch rv.Kind() {
	case reflect.Slice, reflect.Array:
		result := make([]interface{}, 0, rv.Len())
		for i := 0; i < rv.Len(); i++ {
			result = append(result, normalizeABIValue(rv.Index(i).Interface()))
		}
		return result
	case reflect.Struct:
		rt := rv.Type()
		result := make(map[string]interface{}, rv.NumField())
		for i := 0; i < rv.NumField(); i++ {
			field := rt.Field(i)
			if field.PkgPath != "" {
				continue
			}
			name := field.Tag.Get("abi")
			if name == "" {
				name = lowerFirst(field.Name)
			}
			result[name] = normalizeABIValue(rv.Field(i).Interface())
		}
		return result
	case reflect.Map:
		result := map[string]interface{}{}
		iter := rv.MapRange()
		for iter.Next() {
			result[fmt.Sprintf("%v", iter.Key().Interface())] = normalizeABIValue(iter.Value().Interface())
		}
		return result
	default:
		return fmt.Sprintf("%v", value)
	}
}

func determineChainEventType(eventName, contractCode string) string {
	switch strings.ToLower(eventName) {
	case "mint":
		return "RT_MINT"
	case "burn":
		return "RT_BURN"
	case "onrampevent":
		return "RT_ON_RAMP"
	case "encashsuspenseaccountreceived":
		return "RT_ENCASH_SUSPENSE"
	case "encashevent":
		return "RT_ENCASH"
	case "offrampevent":
		return "RT_OFF_RAMP"
	case "permissionset":
		return "USER_PERMISSION_SET"
	case "createtrade":
		return "RT_SEND_CREATE"
	case "conditionaccept":
		return "RT_SEND_CONDITION_ACCEPT"
	case "conditionreject":
		return "RT_SEND_CONDITION_REJECT"
	case "conditionpartialaccept":
		return "RT_SEND_CONDITION_PARTIAL_ACCEPT"
	case "conditionsetdate":
		return "RT_SEND_CONDITION_SET_DATE"
	case "settletrade":
		return "RT_SEND_SETTLE"
	case "sendsuspenseaccountreceived":
		return "RT_SEND_SUSPENSE"
	case "rorconvert":
		return "ROR_CONVERT"
	case "rorsendmint":
		return "ROR_SEND_MINT"
	case "rorsplit":
		return "ROR_SPLIT"
	case "rorsendrelationchange":
		return "ROR_SEND_RELATION_CHANGE"
	case "rortransferstatuschange":
		return "ROR_TRANSFER_STATUS_CHANGE"
	case "transfer":
		if strings.Contains(strings.ToUpper(contractCode), "ERC721") {
			return "ROR_TRANSFER"
		}
		if strings.Contains(strings.ToUpper(contractCode), "ERC20") {
			return "RT_TRANSFER"
		}
	}
	return eventName
}

func (s *ethereumService) transformEventData(eventType string, contract *models.ContractInfo, raw string) string {
	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return raw
	}

	tokenCode := s.resolveTokenCode(lowerString(firstNonNil(payload["tokenAddress"], payload["contractAddr"])))

	var transformed interface{}
	switch eventType {
	case "RT_TRANSFER":
		transformed = map[string]interface{}{
			"tokenCode":    tokenCode,
			"tokenAddress": lowerString(payload["contractAddr"]),
			"from":         lowerString(payload["from"]),
			"to":           lowerString(payload["to"]),
			"amount":       decimalString(payload["value"]),
		}
	case "ROR_TRANSFER":
		transformed = map[string]interface{}{
			"tokenAddress": lowerString(payload["contractAddr"]),
			"from":         lowerString(payload["from"]),
			"to":           lowerString(payload["to"]),
			"tokenId":      decimalString(firstNonNil(payload["tokenId"], payload["value"])),
		}
	case "RT_MINT":
		transformed = map[string]interface{}{
			"bid":       stringValue(firstNonNil(payload["txID"], payload["bid"], payload["businessId"])),
			"tokenCode": tokenCode,
			"recipient": lowerString(firstNonNil(payload["recipient"], payload["to"])),
			"amount":    decimalString(firstNonNil(payload["amount"], payload["value"])),
		}
	case "RT_BURN":
		transformed = map[string]interface{}{
			"bid":       stringValue(firstNonNil(payload["txID"], payload["bid"], payload["businessId"])),
			"tokenCode": tokenCode,
			"owner":     lowerString(firstNonNil(payload["owner"], payload["from"])),
			"amount":    decimalString(firstNonNil(payload["amount"], payload["value"])),
		}
	case "RT_ON_RAMP":
		transformed = map[string]interface{}{
			"bid":          stringValue(payload["bid"]),
			"tokenAddress": lowerString(firstNonNil(payload["tokenAddress"], payload["contractAddr"])),
			"tokenCode":    tokenCode,
			"requester":    lowerString(payload["requester"]),
			"value":        decimalString(firstNonNil(payload["value"], payload["amount"])),
			"state":        stringValue(payload["state"]),
			"extension":    stringValue(payload["extension"]),
		}
	case "RT_OFF_RAMP", "RT_ENCASH":
		transformed = map[string]interface{}{
			"bid":             stringValue(payload["bid"]),
			"tokenCode":       tokenCode,
			"tokenAddress":    lowerString(firstNonNil(payload["tokenAddress"], payload["contractAddr"])),
			"contractAddress": lowerString(payload["contractAddr"]),
			"encasher":        lowerString(firstNonNil(payload["requester"], payload["encasher"])),
			"amount":          decimalString(firstNonNil(payload["value"], payload["amount"])),
			"encashStatus":    stringValue(firstNonNil(payload["state"], payload["encashStatus"])),
			"extension":       stringValue(payload["extension"]),
		}
	case "RT_SEND_SETTLE":
		status := decimalString(payload["status"])
		settleStatus := status
		if status == "1" {
			settleStatus = "SEND"
		} else if status == "2" {
			settleStatus = "REFUND"
		}
		transformed = map[string]interface{}{
			"bid":       stringValue(firstNonNil(payload["businessId"], payload["bid"])),
			"status":    settleStatus,
			"extension": stringValue(payload["extension"]),
		}
	case "RT_SEND_CREATE":
		transformed = map[string]interface{}{
			"bid":                  stringValue(firstNonNil(payload["businessId"], payload["bid"])),
			"tokenCode":            tokenCode,
			"tokenAddress":         lowerString(payload["tokenAddress"]),
			"sendContractAddress":  lowerString(payload["contractAddr"]),
			"creator":              lowerString(payload["creator"]),
			"receiver":             lowerString(payload["receiver"]),
			"value":                decimalString(firstNonNil(payload["tokenAmount"], payload["value"])),
			"timeScId":             stringValue(payload["timeScId"]),
			"conditionSetId":       stringValue(firstNonNil(payload["conditionSetId"], payload["csId"])),
			"parentBusinessId":     stringValue(payload["parentBusinessId"]),
			"partialAcceptEnable":  boolValue(payload["partialAcceptEnable"]),
			"partialAcceptAddress": lowerString(payload["partialAcceptAddress"]),
			"partialAcceptScId":    stringValue(payload["partialAcceptScId"]),
			"scSet":                convertSingleConditions(payload["scSet"]),
			"csSet":                convertConditionSets(firstNonNil(payload["csSet"], payload["css"])),
			"extension":            stringValue(payload["extension"]),
		}
	case "RT_SEND_CONDITION_ACCEPT", "RT_SEND_CONDITION_REJECT":
		transformed = map[string]interface{}{
			"bid":          stringValue(firstNonNil(payload["businessId"], payload["bid"])),
			"scId":         stringValue(payload["scId"]),
			"commentsHash": stringValue(payload["commentsHash"]),
			"filesHash":    stringSlice(payload["filesHash"]),
			"extension":    stringValue(payload["extension"]),
		}
	case "RT_SEND_CONDITION_PARTIAL_ACCEPT":
		transformed = map[string]interface{}{
			"bid":             stringValue(firstNonNil(payload["businessId"], payload["bid"])),
			"subTradeId":      stringValue(firstNonNil(payload["subBusinessId"], payload["subTradeId"])),
			"acceptedAmount":  decimalString(payload["acceptedAmount"]),
			"remainingAmount": decimalString(payload["remainingAmount"]),
			"extension":       stringValue(payload["extension"]),
		}
	case "RT_SEND_CONDITION_SET_DATE":
		transformed = map[string]interface{}{
			"bid":          stringValue(firstNonNil(payload["businessId"], payload["bid"])),
			"scId":         stringValue(payload["scId"]),
			"setValue":     stringValue(payload["setValue"]),
			"commentsHash": stringValue(payload["commentsHash"]),
			"filesHash":    stringSlice(payload["filesHash"]),
			"extension":    stringValue(payload["extension"]),
		}
	case "RT_SEND_SUSPENSE":
		transformed = map[string]interface{}{
			"bid":                stringValue(firstNonNil(payload["businessID"], payload["businessId"], payload["bid"])),
			"tokenCode":          tokenCode,
			"receiverAddress":    lowerString(payload["receiverAddress"]),
			"receiverPermission": decimalString(payload["receiverPermission"]),
			"value":              decimalString(firstNonNil(payload["amount"], payload["value"])),
			"reason":             stringValue(payload["reason"]),
			"extension":          stringValue(payload["extension"]),
		}
	case "RT_ENCASH_SUSPENSE":
		transformed = map[string]interface{}{
			"bid":                stringValue(firstNonNil(payload["businessID"], payload["businessId"], payload["bid"])),
			"tokenCode":          tokenCode,
			"receiver":           lowerString(firstNonNil(payload["receiverAddress"], payload["receiver"])),
			"receiverPermission": decimalString(payload["receiverPermission"]),
			"amount":             decimalString(firstNonNil(payload["amount"], payload["value"])),
			"reason":             stringValue(payload["reason"]),
		}
	case "ROR_CONVERT":
		transformed = map[string]interface{}{
			"bid":                lowerString(firstNonNil(payload["contractAddr"], payload["rorAddress"], payload["bid"])),
			"rorContractAddress": lowerString(firstNonNil(payload["rorAddress"], payload["contractAddr"])),
			"rorId":              decimalString(payload["rorId"]),
			"ownerAddress":       lowerString(firstNonNil(payload["owner"], payload["ownerAddress"])),
			"dttAddress":         lowerString(payload["dttAddress"]),
			"settleStatus":       decimalString(payload["settleStatus"]),
			"extension":          stringValue(payload["extension"]),
		}
	case "ROR_SEND_MINT":
		transformed = map[string]interface{}{
			"bid":                stringValue(firstNonNil(payload["sendRefId"], payload["bid"])),
			"rorContractAddress": lowerString(firstNonNil(payload["rorAddress"], payload["contractAddr"])),
			"rorId":              decimalString(payload["rorId"]),
			"ownerAddress":       lowerString(firstNonNil(payload["owner"], payload["ownerAddress"])),
			"rorValue":           decimalString(firstNonNil(payload["value"], payload["rorValue"])),
			"startAmountLine":    decimalString(payload["startAmountLine"]),
			"endAmountLine":      decimalString(payload["endAmountLine"]),
			"extension":          stringValue(payload["extension"]),
		}
	case "ROR_SPLIT":
		transformed = map[string]interface{}{
			"bid":                lowerString(firstNonNil(payload["rorAddress"], payload["contractAddr"], payload["bid"])),
			"rorContractAddress": lowerString(firstNonNil(payload["rorAddress"], payload["contractAddr"])),
			"rorId":              decimalString(payload["rorId"]),
			"ownerAddress":       lowerString(firstNonNil(payload["rorOwner"], payload["ownerAddress"])),
			"rorValue":           decimalString(payload["rorValue"]),
			"splitRorId":         decimalString(payload["splitRorId"]),
			"splitRorValue":      decimalString(payload["splitRorValue"]),
			"remainRorId":        decimalString(payload["remainRorId"]),
			"remainRorValue":     decimalString(payload["remainRorValue"]),
			"splitType":          decimalString(payload["splitType"]),
			"extension":          stringValue(payload["extension"]),
		}
	case "ROR_SEND_RELATION_CHANGE":
		transformed = map[string]interface{}{
			"bid":                lowerString(firstNonNil(payload["rorAddress"], payload["contractAddr"], payload["bid"])),
			"rorContractAddress": lowerString(firstNonNil(payload["rorAddress"], payload["contractAddr"])),
			"rorId":              decimalString(payload["rorId"]),
			"oldSendRefId":       stringValue(payload["oldSendRefId"]),
			"newSendRefId":       stringValue(payload["newSendRefId"]),
			"extension":          stringValue(payload["extension"]),
		}
	case "ROR_TRANSFER_STATUS_CHANGE":
		transformed = map[string]interface{}{
			"bid":                  stringValue(firstNonNil(payload["transferRefId"], payload["bid"])),
			"rorContractAddress":   lowerString(firstNonNil(payload["rorAddress"], payload["contractAddr"])),
			"rorId":                decimalString(payload["rorId"]),
			"fromAddress":          lowerString(firstNonNil(payload["from"], payload["fromAddress"])),
			"toAddress":            lowerString(firstNonNil(payload["to"], payload["toAddress"])),
			"considerationType":    decimalString(payload["considerationType"]),
			"considerationAddress": lowerString(firstNonNil(payload["considerationAddress"], payload["considerationDttAddr"])),
			"considerationValue":   decimalString(firstNonNil(payload["considerationValue"], payload["considerationAmount"])),
			"transferStatus":       decimalString(payload["transferStatus"]),
			"extension":            stringValue(payload["extension"]),
		}
	default:
		return raw
	}

	out, err := json.Marshal(transformed)
	if err != nil {
		return raw
	}
	return string(out)
}

func (s *ethereumService) resolveTokenCode(address string) string {
	if address == "" {
		return ""
	}
	token, err := s.tokens.FindTokenByAddress(address)
	if err != nil || token == nil {
		return ""
	}
	return token.TokenCode
}

func convertSingleConditions(value interface{}) []map[string]interface{} {
	items := interfaceSlice(value)
	result := make([]map[string]interface{}, 0, len(items))
	for _, item := range items {
		m := mapValue(item)
		if len(m) == 0 {
			continue
		}
		result = append(result, map[string]interface{}{
			"id":             stringValue(m["id"]),
			"conditionType":  stringValue(m["conditionType"]),
			"description":    stringValue(m["description"]),
			"fixFactors":     convertConditionFactors(firstNonNil(m["fixFactors"], m["fixFactorList"])),
			"dynamicFactors": convertConditionFactors(firstNonNil(m["dynamicFactors"], m["dynamicFactorList"])),
		})
	}
	return result
}

func convertConditionFactors(value interface{}) []map[string]interface{} {
	items := interfaceSlice(value)
	result := make([]map[string]interface{}, 0, len(items))
	for _, item := range items {
		m := mapValue(item)
		if len(m) == 0 {
			continue
		}
		result = append(result, map[string]interface{}{
			"name":                stringValue(m["name"]),
			"value":               stringValue(m["value"]),
			"changeFlag":          boolValue(m["changeFlag"]),
			"changeable":          boolValue(firstNonNil(m["changeAble"], m["changeable"])),
			"changeAddress":       lowerString(firstNonNil(m["changeAddr"], m["changeAddress"])),
			"changeDurationStart": uint64Value(firstNonNil(m["beginTime"], m["changeDurationStart"])),
			"changeDurationEnd":   uint64Value(firstNonNil(m["endTime"], m["changeDurationEnd"])),
			"commentsHash":        stringValue(m["commentsHash"]),
			"filesHash":           stringSlice(m["filesHash"]),
		})
	}
	return result
}

func convertConditionSets(value interface{}) []map[string]interface{} {
	items := interfaceSlice(value)
	result := make([]map[string]interface{}, 0, len(items))
	for _, item := range items {
		m := mapValue(item)
		if len(m) == 0 {
			continue
		}
		join := decimalString(firstNonNil(m["join"], m["joinType"]))
		joinType := join
		if join == "0" {
			joinType = "AND"
		} else if join == "1" {
			joinType = "OR"
		}
		result = append(result, map[string]interface{}{
			"id":       stringValue(m["id"]),
			"scIds":    stringSlice(firstNonNil(m["scIDs"], m["scIds"])),
			"csIds":    stringSlice(firstNonNil(m["csIDs"], m["csIds"])),
			"joinType": joinType,
		})
	}
	return result
}

func interfaceSlice(value interface{}) []interface{} {
	switch v := value.(type) {
	case []interface{}:
		return v
	default:
		return []interface{}{}
	}
}

func mapValue(value interface{}) map[string]interface{} {
	switch v := value.(type) {
	case map[string]interface{}:
		return v
	default:
		return map[string]interface{}{}
	}
}

func stringSlice(value interface{}) []string {
	items := interfaceSlice(value)
	result := make([]string, 0, len(items))
	for _, item := range items {
		result = append(result, stringValue(item))
	}
	return result
}

func firstNonNil(values ...interface{}) interface{} {
	for _, value := range values {
		if value != nil {
			return value
		}
	}
	return nil
}

func lowerString(value interface{}) string {
	return strings.ToLower(stringValue(value))
}

func stringValue(value interface{}) string {
	switch v := value.(type) {
	case nil:
		return ""
	case string:
		return v
	case fmt.Stringer:
		return v.String()
	default:
		return fmt.Sprintf("%v", value)
	}
}

func decimalString(value interface{}) string {
	return stringValue(value)
}

func boolValue(value interface{}) bool {
	switch v := value.(type) {
	case bool:
		return v
	case string:
		return strings.EqualFold(v, "true")
	default:
		return false
	}
}

func uint64Value(value interface{}) uint64 {
	switch v := value.(type) {
	case float64:
		return uint64(v)
	case string:
		b, ok := new(big.Int).SetString(v, 10)
		if ok {
			return b.Uint64()
		}
	}
	return 0
}

func lowerFirst(value string) string {
	if value == "" {
		return value
	}
	return strings.ToLower(value[:1]) + value[1:]
}
