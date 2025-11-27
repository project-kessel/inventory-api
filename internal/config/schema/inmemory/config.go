package inmemory

type Config struct {
	*Options
}

type completedConfig struct {
	Type string
	Path string
}

type CompletedConfig struct {
	*completedConfig
}

func NewConfig(o *Options) *Config {
	return &Config{
		Options: o,
	}
}

func (c *Config) Complete() (CompletedConfig, error) {
	return CompletedConfig{
		&completedConfig{Type: c.Type, Path: c.Path},
	}, nil
}
