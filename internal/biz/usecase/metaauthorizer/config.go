package metaauthorizer

type Config struct {
	*Options
}

type completedConfig struct {
	*Options
	TupleCrudAllowlist []string
}

type CompletedConfig struct {
	*completedConfig
}

func NewConfig(o *Options) *Config {
	return &Config{Options: o}
}

func (c *Config) Complete() (CompletedConfig, []error) {
	return CompletedConfig{&completedConfig{
		Options:            c.Options,
		TupleCrudAllowlist: c.TupleCrudAllowlist,
	}}, nil
}
