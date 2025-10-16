package schemas

import "github.com/project-kessel/inventory-api/internal/schemas/in_memory"

type Config struct {
	Repository string
	InMemory   *in_memory.Config
}

type completedConfig struct {
	Repository string
	InMemory   in_memory.CompletedConfig
}

type CompletedConfig struct {
	*completedConfig
}

func NewConfig(o *Options) *Config {
	cfg := &Config{
		Repository: o.Repository,
	}

	if cfg.Repository == InMemoryRepository {
		cfg.InMemory = in_memory.NewConfig(o.InMemory)
	}

	return cfg
}

func (c *Config) Complete() (CompletedConfig, []error) {
	cfg := &completedConfig{
		Repository: c.Repository,
	}

	if c.Repository == InMemoryRepository {
		if inMemory, err := c.InMemory.Complete(); err != nil {
			return CompletedConfig{}, []error{err}
		} else {
			cfg.InMemory = inMemory
		}
	}

	return CompletedConfig{cfg}, nil
}
