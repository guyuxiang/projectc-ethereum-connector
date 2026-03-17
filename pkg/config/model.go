package config

type Config struct {
	Server    *Server    `yaml:"server"`
	Auth      *Auth      `yaml:"auth"`
	Gin       *Gin       `yaml:"gin"`
	Log       *Log       `yaml:"log"`
	MySQL     *MySQL     `yaml:"mysql"`
	Callback  *Callback  `yaml:"callback"`
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

type Callback struct {
	Mode            string `yaml:"mode"`
	TxHTTPURL       string `yaml:"txHttpUrl"`
	RollbackHTTPURL string `yaml:"rollbackHttpUrl"`
}

type Ethereum struct {
	Network   NetworkConfig    `yaml:"network"`
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
	Address           string `yaml:"address"`
	ABI               string `yaml:"abi"`
	DeployBlockNumber uint64 `yaml:"deployBlockNumber"`
}

type Connector struct {
	RequireAuthOnBusiness bool         `yaml:"requireAuthOnBusiness"`
	Wallet                WalletSigner `yaml:"wallet"`
}

type WalletSigner struct {
	FromAddress string `yaml:"fromAddress"`
	PrivateKey  string `yaml:"privateKey"`
}
