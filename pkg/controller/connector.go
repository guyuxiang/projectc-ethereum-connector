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
	contracts     service.ContractRegistryService
	subscriptions service.SubscriptionService
	onchain       service.OnchainRecordService
	wallet        service.WalletService
	scplus        service.ScplusService
}

func NewConnectorController() *ConnectorController {
	contracts := service.NewContractRegistryService()
	onchain := service.NewOnchainRecordService()
	eth := service.NewEthereumService(contracts)
	return &ConnectorController{
		eth:           eth,
		contracts:     contracts,
		subscriptions: service.NewSubscriptionService(eth),
		onchain:       onchain,
		wallet:        service.NewWalletService(onchain),
		scplus:        service.NewScplusService(contracts, onchain),
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

func (ctl *ConnectorController) TxSend(c *gin.Context) {
	var req models.TxSendRequest
	if !bindJSON(c, &req) {
		return
	}
	txHash, err := ctl.eth.SendRawTransaction(c.Request.Context(), c.Param("networkCode"), req.TxSignResult)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, models.Success(models.TxSendResponse{TxCode: txHash}))
}

func (ctl *ConnectorController) TxQuery(c *gin.Context) {
	var req models.TxQueryRequest
	if !bindJSON(c, &req) {
		return
	}
	resp, err := ctl.eth.QueryTransaction(c.Request.Context(), c.Param("networkCode"), req.TxCode)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, models.Success(resp))
}

func (ctl *ConnectorController) AddressBalance(c *gin.Context) {
	var req models.AddressBalanceRequest
	if !bindJSON(c, &req) {
		return
	}
	resp, err := ctl.eth.GetAddressBalance(c.Request.Context(), c.Param("networkCode"), req.Address)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, models.Success(resp))
}

func (ctl *ConnectorController) LatestBlock(c *gin.Context) {
	resp, err := ctl.eth.GetLatestBlock(c.Request.Context(), c.Param("networkCode"))
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, models.Success(resp))
}

func (ctl *ConnectorController) TokenSupply(c *gin.Context) {
	var req models.TokenSupplyRequest
	if !bindJSON(c, &req) {
		return
	}
	resp, err := ctl.eth.GetTokenSupply(c.Request.Context(), c.Param("networkCode"), req.TokenCode)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, models.Success(resp))
}

func (ctl *ConnectorController) TokenBalance(c *gin.Context) {
	var req models.TokenBalanceRequest
	if !bindJSON(c, &req) {
		return
	}
	resp, err := ctl.eth.GetTokenBalance(c.Request.Context(), c.Param("networkCode"), req.TokenCode, req.Address)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, models.Success(resp))
}

func (ctl *ConnectorController) Faucet(c *gin.Context) {
	var req models.BalanceChargeRequest
	if !bindJSON(c, &req) {
		return
	}
	record, err := ctl.wallet.CreateNativeCharge(c.Request.Context(), c.Param("networkCode"), req)
	if err != nil {
		writeError(c, err)
		return
	}
	_ = ctl.onchain.Submit(c.Request.Context(), record.TxCode)
	c.JSON(http.StatusOK, models.Success(record))
}

func (ctl *ConnectorController) DttSendSettle(c *gin.Context) {
	var req models.SettleRequest
	if !bindJSON(c, &req) {
		return
	}
	record, err := ctl.scplus.DttSendSettle(c.Request.Context(), c.Param("networkCode"), req)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, models.Success(record))
}

func (ctl *ConnectorController) AutoReject(c *gin.Context) {
	var req models.AutoRejectRequest
	if !bindJSON(c, &req) {
		return
	}
	record, err := ctl.scplus.AutoReject(c.Request.Context(), c.Param("networkCode"), req)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, models.Success(record))
}

func (ctl *ConnectorController) InstantOnRamp(c *gin.Context) {
	var req models.InstantOnRampRequest
	if !bindJSON(c, &req) {
		return
	}
	record, err := ctl.scplus.InstantOnRamp(c.Request.Context(), c.Param("networkCode"), req)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, models.Success(record))
}

func (ctl *ConnectorController) Issue(c *gin.Context) {
	var req models.IssueInvokeRequest
	if !bindJSON(c, &req) {
		return
	}
	record, err := ctl.scplus.Issue(c.Request.Context(), c.Param("networkCode"), req)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, models.Success(record))
}

func (ctl *ConnectorController) QueryIssue(c *gin.Context) {
	ctl.queryOnchain(c, "SCPLUS_ISSUE")
}

func (ctl *ConnectorController) Finance(c *gin.Context) {
	var req models.FinanceInvokeRequest
	if !bindJSON(c, &req) {
		return
	}
	record, err := ctl.scplus.Finance(c.Request.Context(), c.Param("networkCode"), req)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, models.Success(record))
}

func (ctl *ConnectorController) QueryFinance(c *gin.Context) {
	ctl.queryOnchain(c, "SCPLUS_FINANCE")
}

func (ctl *ConnectorController) IssueF(c *gin.Context) {
	var req models.IssueAndFinanceInvokeRequest
	if !bindJSON(c, &req) {
		return
	}
	record, err := ctl.scplus.IssueF(c.Request.Context(), c.Param("networkCode"), req)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, models.Success(record))
}

func (ctl *ConnectorController) QueryIssueF(c *gin.Context) {
	ctl.queryOnchain(c, "SCPLUS_ISSUE_AND_FINANCE")
}

func (ctl *ConnectorController) TxSubscribe(c *gin.Context) {
	var req models.TxSubscribeRequest
	if !bindJSON(c, &req) {
		return
	}
	ctl.subscriptions.AddTx(c.Param("networkCode"), req)
	c.JSON(http.StatusOK, models.Success(struct{}{}))
}

func (ctl *ConnectorController) AddressSubscribe(c *gin.Context) {
	var req models.AddressSubscribeRequest
	if !bindJSON(c, &req) {
		return
	}
	ctl.subscriptions.AddAddress(c.Param("networkCode"), req)
	c.JSON(http.StatusOK, models.Success(struct{}{}))
}

func (ctl *ConnectorController) TxSubscribeCancel(c *gin.Context) {
	var req models.TxSubscribeCancelRequest
	if !bindJSON(c, &req) {
		return
	}
	ctl.subscriptions.RemoveTx(c.Param("networkCode"), req.TxCode)
	c.JSON(http.StatusOK, models.Success(struct{}{}))
}

func (ctl *ConnectorController) AddressSubscribeCancel(c *gin.Context) {
	var req models.AddressSubscribeCancelRequest
	if !bindJSON(c, &req) {
		return
	}
	ctl.subscriptions.RemoveAddress(c.Param("networkCode"), req)
	c.JSON(http.StatusOK, models.Success(struct{}{}))
}

func (ctl *ConnectorController) ContractList(c *gin.Context) {
	c.JSON(http.StatusOK, models.Success(models.ContractListResponse{
		ContractInfos: ctl.contracts.ListContracts(c.Param("networkCode")),
	}))
}

func (ctl *ConnectorController) Web3ContractInfo(c *gin.Context) {
	c.JSON(http.StatusOK, models.Success(models.Web3ContractInfoResponse{
		Web3ContractInfos: ctl.contracts.ListWeb3Contracts(),
	}))
}

func (ctl *ConnectorController) ContractPush(c *gin.Context) {
	var req models.ContractConfigPushMessage
	if !bindJSON(c, &req) {
		return
	}
	ctl.contracts.Push(req)
	c.JSON(http.StatusOK, models.Success(struct{}{}))
}

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

func (ctl *ConnectorController) ContractPushPage(c *gin.Context) {
	var req models.PageRequest[models.ContractConfigPushRecordQuery]
	if !bindJSON(c, &req) {
		return
	}
	c.JSON(http.StatusOK, models.Success(ctl.contracts.PagePushRecords(req)))
}

func (ctl *ConnectorController) queryOnchain(c *gin.Context, kind string) {
	var req models.BusinessQueryRequest
	if !bindJSON(c, &req) {
		return
	}
	resp, err := ctl.onchain.Get(kind, req.BusinessID)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, models.Success(resp))
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
