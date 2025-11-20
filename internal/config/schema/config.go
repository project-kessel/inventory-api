package schema

import (
	"github.com/project-kessel/inventory-api/internal/config/schema/inmemory"
)

type Config struct {
	Repository string
	InMemory   *inmemory.Config
}

type completedConfig struct {
	Repository string
	InMemory   inmemory.CompletedConfig
}

type CompletedConfig struct {
	*completedConfig
}

func NewConfig(o *Options) *Config {
	cfg := &Config{
		Repository: o.Repository,
	}

	if cfg.Repository == InMemoryRepository {
		cfg.InMemory = inmemory.NewConfig(o.InMemory)
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
