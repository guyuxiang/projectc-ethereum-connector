package store

import "time"

type ContractConfigPushRecordPO struct {
	ID          uint   `gorm:"primaryKey"`
	Code        string `gorm:"size:128;uniqueIndex;not null"`
	Description string `gorm:"size:1024"`
	PushItems   string `gorm:"type:longtext;not null"`
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type CurrentContractConfigPO struct {
	ID                      uint   `gorm:"primaryKey"`
	Code                    string `gorm:"size:191;uniqueIndex;not null"`
	NetworkCode             string `gorm:"size:128;index;not null"`
	ContractCode            string `gorm:"size:128;index;not null"`
	ContractAddress         string `gorm:"size:256;index;not null"`
	ContractABI             string `gorm:"type:longtext"`
	ContractDeployTxBlockNo uint64 `gorm:"not null"`
	ApplyHistory            string `gorm:"type:longtext;not null"`
	CreatedAt               time.Time
	UpdatedAt               time.Time
}

type OnchainRecordPO struct {
	ID                    uint   `gorm:"primaryKey"`
	Code                  string `gorm:"size:191;uniqueIndex;not null"`
	IdempotencyKey        string `gorm:"size:191;not null;index:idx_onchain_type_key,unique"`
	SignerAddress         string `gorm:"size:256"`
	OnchainType           string `gorm:"size:128;not null;index:idx_onchain_type_key,unique"`
	OnchainStatus         string `gorm:"size:64;not null;index"`
	LastError             string `gorm:"size:1024"`
	RetryCount            uint64 `gorm:"not null;default:0"`
	RequestBusiData       string `gorm:"type:longtext"`
	ResponseBusiData      string `gorm:"type:longtext"`
	NetworkCode           string `gorm:"size:128;index"`
	ChainTxData           string `gorm:"type:longtext"`
	SignedTransactionData string `gorm:"type:longtext"`
	RawTransactionData    string `gorm:"type:longtext"`
	TxCode                string `gorm:"size:191;index"`
	Nonce                 uint64
	CreatedAt             time.Time
	UpdatedAt             time.Time
}

type SignerNoncePO struct {
	ID            uint   `gorm:"primaryKey"`
	SignerAddress string `gorm:"size:191;not null;index:idx_signer_network,unique"`
	NetworkCode   string `gorm:"size:128;not null;index:idx_signer_network,unique"`
	CurrentNonce  uint64 `gorm:"not null"`
	SyncTimestamp int64  `gorm:"not null"`
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type TxSubscriptionPO struct {
	ID           uint   `gorm:"primaryKey"`
	Code         string `gorm:"size:191;uniqueIndex;not null"`
	TxCode       string `gorm:"size:191;index;not null"`
	NetworkCode  string `gorm:"size:128;index;not null"`
	EndTimestamp int64  `gorm:"index;not null"`
	Status       string `gorm:"size:64;index;not null"`
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type AddressSubscriptionPO struct {
	ID               uint   `gorm:"primaryKey"`
	Code             string `gorm:"size:191;uniqueIndex;not null"`
	Address          string `gorm:"size:191;index;not null"`
	NetworkCode      string `gorm:"size:128;index;not null"`
	StartBlockNumber uint64 `gorm:"not null"`
	EndBlockNumber   uint64 `gorm:"not null"`
	NextBlockNumber  uint64 `gorm:"not null"`
	Status           string `gorm:"size:64;index;not null"`
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

type AddressSyncWaitingCheckPO struct {
	ID                uint   `gorm:"primaryKey"`
	Code              string `gorm:"size:191;uniqueIndex;not null"`
	NetworkCode       string `gorm:"size:128;index;not null"`
	StartBlockNumber  uint64 `gorm:"not null"`
	EndBlockNumber    uint64 `gorm:"not null"`
	AddressSet        string `gorm:"type:longtext;not null"`
	TxCodeSet         string `gorm:"type:longtext;not null"`
	CheckStatus       string `gorm:"size:64;index;not null"`
	ConfirmBlockCount uint64 `gorm:"not null;default:0"`
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

type TxCallbackRecordPO struct {
	ID                uint   `gorm:"primaryKey"`
	Code              string `gorm:"size:191;uniqueIndex;not null"`
	TxCode            string `gorm:"size:191;index;not null"`
	NetworkCode       string `gorm:"size:128;index;not null"`
	Payload           string `gorm:"type:longtext;not null"`
	PayloadHash       string `gorm:"size:128;index"`
	Status            string `gorm:"size:64;index;not null"`
	CheckStatus       string `gorm:"size:64;index;not null"`
	ConfirmBlockCount uint64 `gorm:"not null;default:0"`
	RetryCount        uint64 `gorm:"not null;default:0"`
	NextRetryAt       int64  `gorm:"index;not null;default:0"`
	LastError         string `gorm:"size:1024"`
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

type TokenRegistryPO struct {
	ID           uint   `gorm:"primaryKey"`
	Code         string `gorm:"size:191;uniqueIndex;not null"`
	NetworkCode  string `gorm:"size:128;index;not null"`
	TokenAddress string `gorm:"size:191;not null"`
	Decimals     int    `gorm:"not null;default:18"`
	CreatedAt    time.Time
	UpdatedAt    time.Time
}
