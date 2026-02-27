package consistency

type Config struct {
	*Options
}

type completedConfig struct {
	*Options

	ReadAfterWriteEnabled          bool
	ReadAfterWriteAllowlist        []string
	DefaultToAtLeastAsAcknowledged bool
}

type CompletedConfig struct {
	*completedConfig
}

func NewConfig(o *Options) *Config {
	cfg := &Config{
		Options: o,
	}
	return cfg
}

func (c *Config) Complete() (CompletedConfig, []error) {
	return CompletedConfig{&completedConfig{
		Options:                        c.Options,
		ReadAfterWriteEnabled:          c.ReadAfterWriteEnabled,
		ReadAfterWriteAllowlist:        c.ReadAfterWriteAllowlist,
		DefaultToAtLeastAsAcknowledged: c.DefaultToAtLeastAsAcknowledged,
	}}, nil
}
