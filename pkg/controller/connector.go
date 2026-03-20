package controller

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/guyuxiang/projectc-ethereum-connector/pkg/models"
	"github.com/guyuxiang/projectc-ethereum-connector/pkg/service"
)

type ConnectorController struct {
	eth           service.EthereumService
	tokens        service.TokenRegistryService
	contracts     service.ContractRegistryService
	subscriptions service.SubscriptionService
	onchain       service.OnchainRecordService
	wallet        service.WalletService
}

func NewConnectorController() *ConnectorController {
	contracts := service.NewContractRegistryService()
	onchain := service.NewOnchainRecordService()
	tokens := service.NewTokenRegistryService()
	eth := service.NewEthereumService(contracts, tokens)
	return &ConnectorController{
		eth:           eth,
		tokens:        tokens,
		contracts:     contracts,
		subscriptions: service.NewSubscriptionService(eth),
		onchain:       onchain,
		wallet:        service.NewWalletService(onchain),
	}
}

func (ctl *ConnectorController) StartBackgroundLoop() {
	ticker := time.NewTicker(15 * time.Second)
	go func() {
		for range ticker.C {
			_ = ctl.onchain.Refresh(context.Background())
			_ = ctl.subscriptions.Refresh(context.Background())
		}
	}()
}

// TxSend godoc
// @Summary Send transaction
// @Description Submit either a signed raw EVM transaction or an ERC-4337 UserOperation to the configured RPC or bundler
// @Tags Common
// @Accept json
// @Produce json
// @Param request body models.TxSendRequest true "Signed transaction payload"
// @Success 200 {object} models.Response
// @Failure 400 {object} models.Response
// @Router /inner/chain-invoke/evm/common/tx-send [post]
func (ctl *ConnectorController) TxSend(c *gin.Context) {
	var req models.TxSendRequest
	if !bindJSON(c, &req) {
		return
	}
	resp, err := ctl.eth.SendTransaction(c.Request.Context(), req)
	if err != nil {
		writeError(c, err)
		return
	}
	if resp != nil && resp.TxCode != "" {
		ctl.subscriptions.AddTx(models.TxSubscribeRequest{
			TxCode: resp.TxCode,
			SubscribeRange: models.TxSubscribeRange{
				EndTimestamp: time.Now().Add(24 * time.Hour).UnixMilli(),
			},
		})
	}
	c.JSON(http.StatusOK, models.Success(resp))
}

// TxQuery godoc
// @Summary Query transaction
// @Description Query transaction status and decoded events by transaction hash
// @Tags Common
// @Accept json
// @Produce json
// @Param request body models.TxQueryRequest true "Transaction query payload"
// @Success 200 {object} models.Response
// @Failure 400 {object} models.Response
// @Router /inner/chain-data/evm/common/tx-query [post]
func (ctl *ConnectorController) TxQuery(c *gin.Context) {
	var req models.TxQueryRequest
	if !bindJSON(c, &req) {
		return
	}
	resp, err := ctl.eth.QueryTransaction(c.Request.Context(), req.TxCode)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, models.Success(resp))
}

// AddressBalance godoc
// @Summary Query address balance
// @Description Query native token balance of an address on the configured network
// @Tags Common
// @Accept json
// @Produce json
// @Param request body models.AddressBalanceRequest true "Address balance query payload"
// @Success 200 {object} models.Response
// @Failure 400 {object} models.Response
// @Router /inner/chain-data/evm/common/address-balance [post]
func (ctl *ConnectorController) AddressBalance(c *gin.Context) {
	var req models.AddressBalanceRequest
	if !bindJSON(c, &req) {
		return
	}
	resp, err := ctl.eth.GetAddressBalance(c.Request.Context(), req.Address)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, models.Success(resp))
}

// LatestBlock godoc
// @Summary Query latest block
// @Description Query latest block number and timestamp from the configured network
// @Tags Common
// @Produce json
// @Success 200 {object} models.Response
// @Failure 400 {object} models.Response
// @Router /inner/chain-data/evm/common/latest-block [post]
func (ctl *ConnectorController) LatestBlock(c *gin.Context) {
	resp, err := ctl.eth.GetLatestBlock(c.Request.Context())
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, models.Success(resp))
}

// TokenSupply godoc
// @Summary Query token supply
// @Description Query total supply of a token configured in the database-backed EVM token registry
// @Tags Common
// @Accept json
// @Produce json
// @Param request body models.TokenSupplyRequest true "Token supply query payload"
// @Success 200 {object} models.Response
// @Failure 400 {object} models.Response
// @Router /inner/chain-data/evm/common/token-supply [post]
func (ctl *ConnectorController) TokenSupply(c *gin.Context) {
	var req models.TokenSupplyRequest
	if !bindJSON(c, &req) {
		return
	}
	resp, err := ctl.eth.GetTokenSupply(c.Request.Context(), req.TokenCode)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, models.Success(resp))
}

// TokenBalance godoc
// @Summary Query token balance
// @Description Query token balance of an address for a token configured in the database-backed EVM token registry
// @Tags Common
// @Accept json
// @Produce json
// @Param request body models.TokenBalanceRequest true "Token balance query payload"
// @Success 200 {object} models.Response
// @Failure 400 {object} models.Response
// @Router /inner/chain-data/evm/common/token-balance [post]
func (ctl *ConnectorController) TokenBalance(c *gin.Context) {
	var req models.TokenBalanceRequest
	if !bindJSON(c, &req) {
		return
	}
	resp, err := ctl.eth.GetTokenBalance(c.Request.Context(), req.TokenCode, req.Address)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, models.Success(resp))
}

// TokenAdd godoc
// @Summary Add or update token
// @Description Add a token definition into database-backed EVM token registry.
// @Tags Common
// @Accept json
// @Produce json
// @Param request body models.TokenAddRequest true "Token add request"
// @Success 200 {object} models.Response
// @Failure 400 {object} models.ErrorResponse
// @Router /inner/chain-data/evm/common/token-add [post]
func (ctl *ConnectorController) TokenAdd(c *gin.Context) {
	var req models.TokenAddRequest
	if !bindJSON(c, &req) {
		return
	}
	resp, err := ctl.tokens.Add(req)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, models.Success(resp))
}

// TokenDelete godoc
// @Summary Delete token
// @Description Delete a token definition from database-backed EVM token registry by token code.
// @Tags Common
// @Accept json
// @Produce json
// @Param request body models.TokenDeleteRequest true "Token delete request"
// @Success 200 {object} models.Response
// @Failure 400 {object} models.ErrorResponse
// @Router /inner/chain-data/evm/common/token-delete [post]
func (ctl *ConnectorController) TokenDelete(c *gin.Context) {
	var req models.TokenDeleteRequest
	if !bindJSON(c, &req) {
		return
	}
	if err := ctl.tokens.Delete(req.TokenCode); err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, models.Success(struct{}{}))
}

// TokenGet godoc
// @Summary Get token
// @Description Get a token definition from database-backed EVM token registry by token code.
// @Tags Common
// @Accept json
// @Produce json
// @Param request body models.TokenGetRequest true "Token get request"
// @Success 200 {object} models.Response
// @Failure 400 {object} models.ErrorResponse
// @Router /inner/chain-data/evm/common/token-get [post]
func (ctl *ConnectorController) TokenGet(c *gin.Context) {
	var req models.TokenGetRequest
	if !bindJSON(c, &req) {
		return
	}
	resp, err := ctl.tokens.Get(req.TokenCode)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, models.Success(resp))
}

// TokenList godoc
// @Summary List tokens
// @Description List token definitions from database-backed EVM token registry.
// @Tags Common
// @Accept json
// @Produce json
// @Param request body models.TokenListRequest true "Token list request"
// @Success 200 {object} models.Response
// @Failure 400 {object} models.ErrorResponse
// @Router /inner/chain-data/evm/common/token-list [post]
func (ctl *ConnectorController) TokenList(c *gin.Context) {
	var req models.TokenListRequest
	if !bindJSON(c, &req) {
		return
	}
	resp, err := ctl.tokens.List()
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, models.Success(models.TokenListResponse{Tokens: resp}))
}

// Faucet godoc
// @Summary Send native token
// @Description Create and submit a native token transfer from the configured wallet
// @Tags Wallet
// @Accept json
// @Produce json
// @Param request body models.BalanceChargeRequest true "Wallet transfer payload"
// @Success 200 {object} models.Response
// @Failure 400 {object} models.Response
// @Router /inner/chain-invoke/evm/wallet/faucet [post]
func (ctl *ConnectorController) Faucet(c *gin.Context) {
	var req models.BalanceChargeRequest
	if !bindJSON(c, &req) {
		return
	}
	record, err := ctl.wallet.CreateNativeCharge(c.Request.Context(), req)
	if err != nil {
		writeError(c, err)
		return
	}
	_ = ctl.onchain.Submit(c.Request.Context(), record.TxCode)
	c.JSON(http.StatusOK, models.Success(record))
}

// TxSubscribe godoc
// @Summary Subscribe transaction
// @Description Register a transaction subscription for callback processing
// @Tags Subscription
// @Accept json
// @Produce json
// @Param request body models.TxSubscribeRequest true "Transaction subscription payload"
// @Success 200 {object} models.Response
// @Failure 400 {object} models.Response
// @Router /inner/chain-data-subscribe/evm/tx-subscribe [post]
func (ctl *ConnectorController) TxSubscribe(c *gin.Context) {
	var req models.TxSubscribeRequest
	if !bindJSON(c, &req) {
		return
	}
	ctl.subscriptions.AddTx(req)
	c.JSON(http.StatusOK, models.Success(struct{}{}))
}

// AddressSubscribe godoc
// @Summary Subscribe contract address logs
// @Description Register a contract-address log subscription based on eth_getLogs for callback processing
// @Tags Subscription
// @Accept json
// @Produce json
// @Param request body models.AddressSubscribeRequest true "Contract address subscription payload"
// @Success 200 {object} models.Response
// @Failure 400 {object} models.Response
// @Router /inner/chain-data-subscribe/evm/address-subscribe [post]
func (ctl *ConnectorController) AddressSubscribe(c *gin.Context) {
	var req models.AddressSubscribeRequest
	if !bindJSON(c, &req) {
		return
	}
	ctl.subscriptions.AddAddress(req)
	c.JSON(http.StatusOK, models.Success(struct{}{}))
}

// TxSubscribeCancel godoc
// @Summary Cancel transaction subscription
// @Description Cancel a registered transaction subscription
// @Tags Subscription
// @Accept json
// @Produce json
// @Param request body models.TxSubscribeCancelRequest true "Transaction subscription cancel payload"
// @Success 200 {object} models.Response
// @Failure 400 {object} models.Response
// @Router /inner/chain-data-subscribe/evm/tx-subscribe-cancel [post]
func (ctl *ConnectorController) TxSubscribeCancel(c *gin.Context) {
	var req models.TxSubscribeCancelRequest
	if !bindJSON(c, &req) {
		return
	}
	ctl.subscriptions.RemoveTx(req.TxCode)
	c.JSON(http.StatusOK, models.Success(struct{}{}))
}

// AddressSubscribeCancel godoc
// @Summary Cancel contract address log subscription
// @Description Update or stop a contract-address log subscription
// @Tags Subscription
// @Accept json
// @Produce json
// @Param request body models.AddressSubscribeCancelRequest true "Contract address subscription cancel payload"
// @Success 200 {object} models.Response
// @Failure 400 {object} models.Response
// @Router /inner/chain-data-subscribe/evm/address-subscribe-cancel [post]
func (ctl *ConnectorController) AddressSubscribeCancel(c *gin.Context) {
	var req models.AddressSubscribeCancelRequest
	if !bindJSON(c, &req) {
		return
	}
	ctl.subscriptions.RemoveAddress(req)
	c.JSON(http.StatusOK, models.Success(struct{}{}))
}

// ContractList godoc
// @Summary List current contracts
// @Description Query currently applied contract configurations for the configured network
// @Tags Contract
// @Produce json
// @Success 200 {object} models.Response
// @Failure 400 {object} models.Response
// @Router /inner/contract/list/evm [post]
func (ctl *ConnectorController) ContractList(c *gin.Context) {
	c.JSON(http.StatusOK, models.Success(models.ContractListResponse{
		ContractInfos: ctl.contracts.ListContracts(),
	}))
}

// Web3ContractInfo godoc
// @Summary List web3 contract info
// @Description Query current contract and network info for web3 clients
// @Tags Contract
// @Produce json
// @Success 200 {object} models.Response
// @Failure 400 {object} models.Response
// @Router /open/all-clients/contract/web3-contract-info [post]
func (ctl *ConnectorController) Web3ContractInfo(c *gin.Context) {
	c.JSON(http.StatusOK, models.Success(models.Web3ContractInfoResponse{
		Web3ContractInfos: ctl.contracts.ListWeb3Contracts(),
	}))
}

// ContractPush godoc
// @Summary Push contract config
// @Description Create a contract configuration push record
// @Tags Contract
// @Accept json
// @Produce json
// @Param request body models.ContractConfigPushMessage true "Contract config push payload"
// @Success 200 {object} models.Response
// @Failure 400 {object} models.Response
// @Router /inner/contract/contract-config-push-record/push [post]
func (ctl *ConnectorController) ContractPush(c *gin.Context) {
	var req models.ContractConfigPushMessage
	if !bindJSON(c, &req) {
		return
	}
	ctl.contracts.Push(req)
	c.JSON(http.StatusOK, models.Success(struct{}{}))
}

// ContractPushApply godoc
// @Summary Apply contract config push
// @Description Apply a pushed contract configuration record into current contract config
// @Tags Contract
// @Accept json
// @Produce json
// @Param request body models.ApplyContractConfigPushRecordRequest true "Apply contract push payload"
// @Success 200 {object} models.Response
// @Failure 400 {object} models.Response
// @Router /open/ops-client/contract/contract-config-push-record/apply [post]
func (ctl *ConnectorController) ContractPushApply(c *gin.Context) {
	var req models.ApplyContractConfigPushRecordRequest
	if !bindJSON(c, &req) {
		return
	}
	if err := ctl.contracts.ApplyPush(req.PushRecordCode); err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, models.Success(struct{}{}))
}

// ContractPushPage godoc
// @Summary Page contract config push records
// @Description Query paged contract configuration push records
// @Tags Contract
// @Accept json
// @Produce json
// @Param request body object true "Contract push page query payload"
// @Success 200 {object} models.Response
// @Failure 400 {object} models.Response
// @Router /open/ops-client/contract/contract-config-push-record/page [post]
func (ctl *ConnectorController) ContractPushPage(c *gin.Context) {
	var req models.PageRequest[models.ContractConfigPushRecordQuery]
	if !bindJSON(c, &req) {
		return
	}
	c.JSON(http.StatusOK, models.Success(ctl.contracts.PagePushRecords(req)))
}

func bindJSON(c *gin.Context, out interface{}) bool {
	if err := c.ShouldBindJSON(out); err != nil {
		c.JSON(http.StatusBadRequest, models.Failure(400, err.Error()))
		return false
	}
	return true
}

func writeError(c *gin.Context, err error) {
	status := http.StatusBadRequest
	if errors.Is(err, serviceErrNotFound) {
		status = http.StatusNotFound
	}
	c.JSON(status, models.Failure(status, err.Error()))
}

var serviceErrNotFound = errors.New("not found")
