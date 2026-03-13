package config

type Config struct {
	Server   *Server   `yaml:"server"`
	Auth     *Auth     `yaml:"auth"`
	Gin      *Gin      `yaml:"gin"`
	Log      *Log      `yaml:"log"`
	MySQL    *MySQL    `yaml:"mysql"`
	RabbitMQ *RabbitMQ `yaml:"rabbitmq"`
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
	URL          string `yaml:"url"`
	Exchange     string `yaml:"exchange"`
	ExchangeType string `yaml:"exchangeType"`
	Queue        string `yaml:"queue"`
	RoutingKey   string `yaml:"routingKey"`
}
