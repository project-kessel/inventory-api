package authz

import (
	"context"

	"github.com/project-kessel/inventory-api/internal/authz/spicedb"
)

type Config struct {
	Authz   string
	SpiceDB *spicedb.Config
}

func NewConfig(o *Options) *Config {
	var scfg *spicedb.Config
	if o.Authz == SpiceDB {
		scfg = spicedb.NewConfig(o.SpiceDB)
	}

	return &Config{
		Authz:   o.Authz,
		SpiceDB: scfg,
	}
}

type completedConfig struct {
	Authz   string
	SpiceDB *spicedb.Config
}

type CompletedConfig struct {
	*completedConfig
}

func (c *Config) Complete(ctx context.Context) (CompletedConfig, []error) {
	cfg := &completedConfig{
		Authz: c.Authz,
	}

	if c.Authz == SpiceDB {
		cfg.SpiceDB = c.SpiceDB
	}

	return CompletedConfig{cfg}, nil
}
