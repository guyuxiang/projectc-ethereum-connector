package main

import (
	"fmt"

	"github.com/gin-gonic/gin"
	"github.com/guyuxiang/projectc-ethereum-connector/pkg/config"
	"github.com/guyuxiang/projectc-ethereum-connector/pkg/log"
	"github.com/guyuxiang/projectc-ethereum-connector/pkg/mysql"
	"github.com/guyuxiang/projectc-ethereum-connector/pkg/route"
	"github.com/guyuxiang/projectc-ethereum-connector/pkg/store"
	"github.com/guyuxiang/projectc-ethereum-connector/pkg/util"
)

func main() {
	util.SetupSigusr1Trap()

	mysqlCfg := config.GetConfig().MySQL
	if mysqlCfg == nil || mysqlCfg.DSN == "" {
		log.Fatal("mysql dsn is required")
	}
	if _, err := mysql.Init(mysqlCfg); err != nil {
		log.Fatalf("init mysql failed: %v", err)
	}
	defer func() {
		if err := mysql.Close(); err != nil {
			log.Errorf("close mysql failed: %v", err)
		}
	}()
	log.Infof("mysql initialized successfully")
	if err := store.AutoMigrate(); err != nil {
		log.Fatalf("auto migrate connector tables failed: %v", err)
	}

	r := gin.Default()
	m := config.GetString(config.FLAG_KEY_GIN_MODE)
	gin.SetMode(m)

	route.InstallRoutes(r)
	serverBindAddr := fmt.Sprintf("%s:%d", config.GetString(config.FLAG_KEY_SERVER_HOST), config.GetInt(config.FLAG_KEY_SERVER_PORT))
	log.Infof("Run server at %s", serverBindAddr)
	r.Run(serverBindAddr) // listen and serve
}
