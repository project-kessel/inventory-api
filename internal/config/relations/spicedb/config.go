package spicedb

type Config struct {
	*Options
}

func NewConfig(o *Options) *Config {
	return &Config{Options: o}
}

type completedConfig struct {
	*Options
}

type CompletedConfig struct {
	*completedConfig
}

func (c *Config) Complete() (CompletedConfig, []error) {
	return CompletedConfig{&completedConfig{Options: c.Options}}, nil
}
