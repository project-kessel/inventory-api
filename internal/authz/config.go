package authz

import (
	"context"

	"github.com/project-kessel/inventory-api/internal/authz/kessel"
)

// MetaAuthorizerConfig holds the configuration for meta-authorization middleware
type MetaAuthorizerConfig struct {
	Enabled   bool
	Namespace string
}

type Config struct {
	Authz          string
	Kessel         *kessel.Config
	MetaAuthorizer *MetaAuthorizerConfig
}

func NewConfig(o *Options) *Config {
	var kcfg *kessel.Config
	if o.Authz == Kessel {
		kcfg = kessel.NewConfig(o.Kessel)
	}

	var metaAuthzConfig *MetaAuthorizerConfig
	if o.MetaAuthorizer != nil {
		enabled := true
		if o.MetaAuthorizer.Enabled != nil {
			enabled = *o.MetaAuthorizer.Enabled
		}
		namespace := "rbac"
		if o.MetaAuthorizer.Namespace != "" {
			namespace = o.MetaAuthorizer.Namespace
		}
		metaAuthzConfig = &MetaAuthorizerConfig{
			Enabled:   enabled,
			Namespace: namespace,
		}
	}

	return &Config{
		Authz:          o.Authz,
		Kessel:         kcfg,
		MetaAuthorizer: metaAuthzConfig,
	}
}

type completedConfig struct {
	Authz          string
	Kessel         kessel.CompletedConfig
	MetaAuthorizer *MetaAuthorizerConfig
}

type CompletedConfig struct {
	*completedConfig
}

func (c *Config) Complete(ctx context.Context) (CompletedConfig, []error) {
	cfg := &completedConfig{
		Authz:          c.Authz,
		MetaAuthorizer: c.MetaAuthorizer,
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
