package store

import (
	"errors"

	"github.com/guyuxiang/projectc-ethereum-connector/pkg/mysql"
)

func AutoMigrate() error {
	db := mysql.DB()
	if db == nil {
		return nil
	}
	return db.AutoMigrate(
		&ContractConfigPushRecordPO{},
		&CurrentContractConfigPO{},
		&OnchainRecordPO{},
		&SignerNoncePO{},
		&TxSubscriptionPO{},
		&AddressSubscriptionPO{},
		&AddressSyncWaitingCheckPO{},
		&TxCallbackRecordPO{},
		&TokenRegistryPO{},
	)
}

func RequireDB() error {
	if mysql.DB() == nil {
		return errors.New("mysql is not initialized")
	}
	return nil
}
