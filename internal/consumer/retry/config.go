package retry

type Config struct {
	*Options
}

type completedConfig struct {
	Options *Options
}

// CompletedConfig can be constructed only from Config.Complete
type CompletedConfig struct {
	*completedConfig
}

func NewConfig(o *Options) *Config {
	return &Config{
		Options: o,
	}
}

func (c *Config) Complete() CompletedConfig {
	return CompletedConfig{&completedConfig{Options: c.Options}}
}
