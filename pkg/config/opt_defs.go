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
		optName:         FLAG_KEY_MYSQL_USERNAME,
		optDefaultValue: "",
		optUsage:        "mysql username",
	},
	{
		optName:         FLAG_KEY_MYSQL_PASSWORD,
		optDefaultValue: "",
		optUsage:        "mysql password",
	},
	{
		optName:         FLAG_KEY_MYSQL_HOST,
		optDefaultValue: "",
		optUsage:        "mysql host",
	},
	{
		optName:         FLAG_KEY_MYSQL_PORT,
		optDefaultValue: 3306,
		optUsage:        "mysql port",
	},
	{
		optName:         FLAG_KEY_MYSQL_DATABASE,
		optDefaultValue: "",
		optUsage:        "mysql database",
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
		optName:         FLAG_KEY_MYSQL_MAX_LIFE,
		optDefaultValue: 300,
		optUsage:        "mysql connection max lifetime seconds",
	},
}
