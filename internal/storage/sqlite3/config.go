package sqlite3

type Config struct {
	*Options
}

type completedConfig struct {
	*Config
}

type CompletedConfig struct {
	*completedConfig
}

func NewConfig(o *Options) *Config {
	return &Config{o}
}

func (c *Config) Complete() CompletedConfig {
	return CompletedConfig{&completedConfig{
		c,
	}}
}
