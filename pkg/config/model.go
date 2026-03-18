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
	Maxopenconns   int    `yaml:"maxopenconns"`
	Maxidleconns   int    `yaml:"maxidleconns"`
	Connmaxlifesec int    `yaml:"connmaxlifesec"`
}

type Callback struct {
	Username        string `yaml:"username"`
	Password        string `yaml:"password"`
	Txhttpurl       string `yaml:"txhttpurl"`
	Rollbackhttpurl string `yaml:"rollbackhttpurl"`
}

type NetworkConfig struct {
	Networkcode  string `yaml:"networkcode"`
	Rpcurl       string `yaml:"rpcurl"`
	Bundlerurl   string `yaml:"bundlerurl"`
	Chainid      int64  `yaml:"chainid"`
	Nativesymbol string `yaml:"nativesymbol"`
}

type WalletSigner struct {
	Fromaddress string `yaml:"fromaddress"`
	Privatekey  string `yaml:"privatekey"`
}
