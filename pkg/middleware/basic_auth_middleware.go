package middleware

import (
	"github.com/gin-gonic/gin"
	"github.com/guyuxiang/projectc-ethereum-connector/pkg/config"
)

func BasicAuthMiddleware() gin.HandlerFunc {
	return gin.BasicAuth(gin.Accounts{
		config.GetString(config.FLAG_KEY_AUTH_USERNAME): config.GetString(config.FLAG_KEY_AUTH_PASSWORD),
	})
}
