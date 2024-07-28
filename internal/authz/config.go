package authz

import (
	"context"

	"github.com/project-kessel/inventory-api/internal/authz/kessel"
)

type Config struct {
	Authz  string
	Kessel *kessel.Config
}

func NewConfig(o *Options) *Config {
	var kcfg *kessel.Config
	if o.Authz == Kessel {
		kcfg = kessel.NewConfig(o.Kessel)
	}

	return &Config{
		Authz:  o.Authz,
		Kessel: kcfg,
	}
}

type completedConfig struct {
	Authz  string
	Kessel kessel.CompletedConfig
}

type CompletedConfig struct {
	*completedConfig
}

func (c *Config) Complete(ctx context.Context) (CompletedConfig, []error) {
	cfg := &completedConfig{}

	if c.Authz == Kessel {
		if ksl, errs := c.Kessel.Complete(ctx); errs != nil {
			return CompletedConfig{}, nil
		} else {
			cfg.Kessel = ksl
		}
	}

	return CompletedConfig{cfg}, nil
}
