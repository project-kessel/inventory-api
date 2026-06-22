package relations

import (
	"context"

	"github.com/project-kessel/inventory-api/internal/config/relations/kessel"
	"github.com/project-kessel/inventory-api/internal/config/relations/spicedb"
)

type Config struct {
	Authz   string
	Kessel  *kessel.Config
	SpiceDB *spicedb.Config
}

func NewConfig(o *Options) *Config {
	cfg := &Config{
		Authz: o.Authz,
	}

	if o.Authz == Kessel {
		cfg.Kessel = kessel.NewConfig(o.Kessel)
	}

	if o.Authz == SpiceDB {
		cfg.SpiceDB = spicedb.NewConfig(o.SpiceDB)
	}

	return cfg
}

type completedConfig struct {
	Authz   string
	Kessel  kessel.CompletedConfig
	SpiceDB spicedb.CompletedConfig
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

	if c.Authz == SpiceDB {
		if sdb, errs := c.SpiceDB.Complete(); errs != nil {
			return CompletedConfig{}, errs
		} else {
			cfg.SpiceDB = sdb
		}
	}

	return CompletedConfig{cfg}, nil
}

func CheckRelationsImpl(config CompletedConfig) string {
	switch config.Authz {
	case AllowAll, Kessel, SpiceDB:
		return config.Authz
	default:
		return "unknown"
	}
}
