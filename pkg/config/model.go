package config

type Config struct {
	Server    *Server    `yaml:"server"`
	Auth      *Auth      `yaml:"auth"`
	Gin       *Gin       `yaml:"gin"`
	Log       *Log       `yaml:"log"`
	MySQL     *MySQL     `yaml:"mysql"`
	RabbitMQ  *RabbitMQ  `yaml:"rabbitmq"`
	Ethereum  *Ethereum  `yaml:"ethereum"`
	Connector *Connector `yaml:"connector"`
}

type Server struct {
	Port    int    `yaml:"port"`
	Host    string `yaml:"host"`
	Version string `yaml:"version"`
}

type Auth struct {
	Username string `yaml:"username"`
	Password string `yaml:"password"`
}

type Gin struct {
	Mode string `yaml:"mode"`
}

type Log struct {
	Level string `yaml:"level"`
}

type MySQL struct {
	DSN          string `yaml:"dsn"`
	MaxIdleConns int    `yaml:"maxIdleConns"`
	MaxOpenConns int    `yaml:"maxOpenConns"`
}

type RabbitMQ struct {
	URL                          string `yaml:"url"`
	Exchange                     string `yaml:"exchange"`
	ExchangeType                 string `yaml:"exchangeType"`
	Queue                        string `yaml:"queue"`
	RoutingKey                   string `yaml:"routingKey"`
	TxCallbackExchange           string `yaml:"txCallbackExchange"`
	TxCallbackExchangeType       string `yaml:"txCallbackExchangeType"`
	TxCallbackCancelExchange     string `yaml:"txCallbackCancelExchange"`
	TxCallbackCancelExchangeType string `yaml:"txCallbackCancelExchangeType"`
}

type Ethereum struct {
	Networks  []NetworkConfig  `yaml:"networks"`
	Contracts []ContractConfig `yaml:"contracts"`
}

type NetworkConfig struct {
	Code                  string `yaml:"code"`
	RPCURL                string `yaml:"rpcUrl"`
	ChainID               int64  `yaml:"chainId"`
	BlockchainExplorerURL string `yaml:"blockchainExplorerUrl"`
}

type ContractConfig struct {
	Code              string `yaml:"code"`
	NetworkCode       string `yaml:"networkCode"`
	Address           string `yaml:"address"`
	ABI               string `yaml:"abi"`
	DeployBlockNumber uint64 `yaml:"deployBlockNumber"`
}

type Connector struct {
	RequireAuthOnBusiness bool            `yaml:"requireAuthOnBusiness"`
	Wallets               []WalletSigner  `yaml:"wallets"`
	Onchains              []OnchainSigner `yaml:"onchains"`
}

type WalletSigner struct {
	NetworkCode string `yaml:"networkCode"`
	FromAddress string `yaml:"fromAddress"`
	PrivateKey  string `yaml:"privateKey"`
}

type OnchainSigner struct {
	NetworkCode         string `yaml:"networkCode"`
	OnchainType         string `yaml:"onchainType"`
	PrivateKey          string `yaml:"privateKey"`
	ContractCode        string `yaml:"contractCode"`
	DefaultTokenAddress string `yaml:"defaultTokenAddress"`
}
