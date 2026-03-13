package main

import (
	"fmt"

	"github.com/gin-gonic/gin"
	"github.com/guyuxiang/projectc-ethereum-connector/pkg/config"
	"github.com/guyuxiang/projectc-ethereum-connector/pkg/log"
	"github.com/guyuxiang/projectc-ethereum-connector/pkg/mysql"
	"github.com/guyuxiang/projectc-ethereum-connector/pkg/rabbitmq"
	"github.com/guyuxiang/projectc-ethereum-connector/pkg/route"
	"github.com/guyuxiang/projectc-ethereum-connector/pkg/store"
	"github.com/guyuxiang/projectc-ethereum-connector/pkg/util"
)

func main() {
	util.SetupSigusr1Trap()

	if mysqlCfg := config.GetConfig().MySQL; mysqlCfg != nil && mysqlCfg.DSN != "" {
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
	} else {
		log.Infof("mysql initialization skipped")
	}

	if rabbitCfg := config.GetConfig().RabbitMQ; rabbitCfg != nil && rabbitCfg.URL != "" {
		if _, err := rabbitmq.Init(rabbitCfg); err != nil {
			log.Fatalf("init rabbitmq failed: %v", err)
		}
		defer func() {
			if err := rabbitmq.Close(); err != nil {
				log.Errorf("close rabbitmq failed: %v", err)
			}
		}()
		log.Infof("rabbitmq initialized successfully")
	} else {
		log.Infof("rabbitmq initialization skipped")
	}

	r := gin.Default()
	m := config.GetString(config.FLAG_KEY_GIN_MODE)
	gin.SetMode(m)

	route.InstallRoutes(r)
	serverBindAddr := fmt.Sprintf("%s:%d", config.GetString(config.FLAG_KEY_SERVER_HOST), config.GetInt(config.FLAG_KEY_SERVER_PORT))
	log.Infof("Run server at %s", serverBindAddr)
	r.Run(serverBindAddr) // listen and serve
}
