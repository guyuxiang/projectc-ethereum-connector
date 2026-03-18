package config

type Config struct {
	Server   *Server        `yaml:"server"`
	Auth     *Auth          `yaml:"auth"`
	Gin      *Gin           `yaml:"gin"`
	Log      *Log           `yaml:"log"`
	MySQL    *MySQL         `yaml:"mysql"`
	Callback *Callback      `yaml:"callback"`
	Network  *NetworkConfig `yaml:"network"`
	Wallet   *WalletSigner  `yaml:"wallet"`
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
	Username       string `yaml:"username"`
	Password       string `yaml:"password"`
	Host           string `yaml:"host"`
	Port           int    `yaml:"port"`
	Database       string `yaml:"database"`
	MaxOpenConns   int    `yaml:"maxOpenconns"`
	MaxIdleConns   int    `yaml:"maxIdleConns"`
	ConnMaxLifeSec int    `yaml:"connMaxLifeSec"`
}

type Callback struct {
	Username        string `yaml:"username"`
	Password        string `yaml:"password"`
	TxHTTPURL       string `yaml:"txHttpUrl"`
	RollbackHTTPURL string `yaml:"rollbackHttpUrl"`
}

type NetworkConfig struct {
	Code         string `yaml:"networkCode"`
	RPCURL       string `yaml:"rpcUrl"`
	BundlerURL   string `yaml:"bundlerUrl"`
	ChainID      int64  `yaml:"chainId"`
	NativeSymbol string `yaml:"nativeSymbol"`
}

type WalletSigner struct {
	FromAddress string `yaml:"fromAddress"`
	PrivateKey  string `yaml:"privateKey"`
}
