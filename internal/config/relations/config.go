package relations

import (
	"context"

	"github.com/project-kessel/inventory-api/internal/config/relations/kessel"
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
	cfg := &completedConfig{
		Authz: c.Authz,
	}

	if c.Authz == Kessel {
		if ksl, errs := c.Kessel.Complete(ctx); errs != nil {
			return CompletedConfig{}, nil
		} else {
			cfg.Kessel = ksl
		}
	}

	return CompletedConfig{cfg}, nil
}

func CheckAuthorizer(config CompletedConfig) string {
	var authType string
	switch config.Authz {
	case AllowAll:
		authType = "AllowAll"
	case Kessel:
		authType = "Kessel"
	default:
		authType = "Unknown"
	}
	return authType
}
