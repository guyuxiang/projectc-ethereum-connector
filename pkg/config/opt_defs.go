package config

var flagsOpts = []flagOpt{
	{
		optName:         FLAG_KEY_SERVER_HOST,
		optDefaultValue: "0.0.0.0",
		optUsage:        "server listen host",
	},
	{
		optName:         FLAG_KEY_SERVER_PORT,
		optDefaultValue: 8081,
		optUsage:        "server listen port",
	},
	{
		optName:         FLAG_KEY_GIN_MODE,
		optDefaultValue: "debug",
		optUsage:        "gin mode",
	},
	{
		optName:         FLAG_KEY_LOG_LEVEL,
		optDefaultValue: "info",
		optUsage:        "log level",
	},
	{
		optName:         FLAG_KEY_MYSQL_DSN,
		optDefaultValue: "",
		optUsage:        "mysql dsn",
	},
	{
		optName:         FLAG_KEY_MYSQL_MAX_IDLE,
		optDefaultValue: 10,
		optUsage:        "mysql max idle connections",
	},
	{
		optName:         FLAG_KEY_MYSQL_MAX_OPEN,
		optDefaultValue: 20,
		optUsage:        "mysql max open connections",
	},
	{
		optName:         FLAG_KEY_RABBIT_URL,
		optDefaultValue: "",
		optUsage:        "rabbitmq connection url",
	},
	{
		optName:         FLAG_KEY_RABBIT_EXCH,
		optDefaultValue: "",
		optUsage:        "rabbitmq exchange",
	},
	{
		optName:         FLAG_KEY_RABBIT_TYPE,
		optDefaultValue: "direct",
		optUsage:        "rabbitmq exchange type",
	},
	{
		optName:         FLAG_KEY_RABBIT_QUEUE,
		optDefaultValue: "",
		optUsage:        "rabbitmq queue",
	},
	{
		optName:         FLAG_KEY_RABBIT_ROUTING,
		optDefaultValue: "",
		optUsage:        "rabbitmq routing key",
	},
}
