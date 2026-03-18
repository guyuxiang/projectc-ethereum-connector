package route

import (
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
	_ "github.com/guyuxiang/projectc-ethereum-connector/docs"
	"github.com/guyuxiang/projectc-ethereum-connector/pkg/controller"
	"github.com/guyuxiang/projectc-ethereum-connector/pkg/log"
	"github.com/guyuxiang/projectc-ethereum-connector/pkg/middleware"
	"github.com/guyuxiang/projectc-ethereum-connector/pkg/models"
	"github.com/guyuxiang/projectc-ethereum-connector/pkg/service"
	ginSwagger "github.com/swaggo/gin-swagger"
	"github.com/swaggo/gin-swagger/swaggerFiles"
)

// @title Swagger projectc-ethereum-connector
// @version 0.1.0
// @description This is a projectc-ethereum-connector.
// @contact.name guyuxiang
// @contact.url https://guyuxiang.github.io
// @contact.email gu_yuxiang@qq.com
// @license.name Apache 2.0
// @license.url http://www.apache.org/licenses/LICENSE-2.0.html
// @BasePath /api/v1
func InstallRoutes(r *gin.Engine) {
	// Recovery middleware recovers from any panics and writes a 500 if there was one.
	r.Use(gin.Recovery())

	// /swagger/index.html
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	// a ping api test
	r.GET("/ping", controller.Ping)

	// get projectc-ethereum-connector version
	r.GET("/version", controller.Version)

	// config reload
	r.Any("/-/reload", func(c *gin.Context) {
		log.Info("===== Server Stop! Cause: Config Reload. =====")
		os.Exit(1)
	})

	secured := r.Group("/api/v1")
	secured.Use(middleware.BasicAuthMiddleware())

	connectorController := controller.NewConnectorController()
	connectorController.StartBackgroundLoop()

	secured.POST("/inner/chain-invoke/evm/common/tx-send", connectorController.TxSend)
	secured.POST("/inner/chain-data/evm/common/tx-query", connectorController.TxQuery)
	secured.POST("/inner/chain-data/evm/common/address-balance", connectorController.AddressBalance)
	secured.POST("/inner/chain-data/evm/common/latest-block", connectorController.LatestBlock)
	secured.POST("/inner/chain-data/evm/common/token-supply", connectorController.TokenSupply)
	secured.POST("/inner/chain-data/evm/common/token-balance", connectorController.TokenBalance)
	secured.POST("/inner/chain-data/evm/common/token-add", connectorController.TokenAdd)
	secured.POST("/inner/chain-data/evm/common/token-delete", connectorController.TokenDelete)
	secured.POST("/inner/chain-data/evm/common/token-get", connectorController.TokenGet)
	secured.POST("/inner/chain-data/evm/common/token-list", connectorController.TokenList)

	secured.POST("/inner/chain-invoke/evm/wallet/faucet", connectorController.Faucet)

	secured.POST("/inner/chain-data-subscribe/evm/tx-subscribe", connectorController.TxSubscribe)
	secured.POST("/inner/chain-data-subscribe/evm/address-subscribe", connectorController.AddressSubscribe)
	secured.POST("/inner/chain-data-subscribe/evm/tx-subscribe-cancel", connectorController.TxSubscribeCancel)
	secured.POST("/inner/chain-data-subscribe/evm/address-subscribe-cancel", connectorController.AddressSubscribeCancel)

	secured.POST("/inner/contract/list/evm", connectorController.ContractList)
	secured.POST("/inner/contract/contract-config-push-record/push", connectorController.ContractPush)

	secured.POST("/open/all-clients/contract/web3-contract-info", connectorController.Web3ContractInfo)
	secured.POST("/open/ops-client/contract/contract-config-push-record/apply", connectorController.ContractPushApply)
	secured.POST("/open/ops-client/contract/contract-config-push-record/page", connectorController.ContractPushPage)

}

func requireConfiguredNetworkCode() gin.HandlerFunc {
	return func(c *gin.Context) {
		configured := service.ConfiguredNetworkCode()
		if configured == "" || c.Param("networkCode") == configured {
			c.Next()
			return
		}
		c.JSON(http.StatusBadRequest, models.Failure(400, "networkCode does not match configured network"))
		c.Abort()
	}
}
