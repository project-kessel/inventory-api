package authn

import (
	"github.com/project-kessel/inventory-api/internal/authn/oidc"
	"github.com/project-kessel/inventory-api/internal/authn/psk"
)

type Config struct {
	AllowUnauthenticated bool

	Oidc          *oidc.Config
	PreSharedKeys *psk.Config
}

func NewConfig(o *Options) *Config {
	cfg := &Config{
		AllowUnauthenticated: o.AllowUnauthenticated,
	}

	if len(o.Oidc.AuthorizationServerURL) > 0 {
		cfg.Oidc = oidc.NewConfig(o.Oidc)
	}

	if len(o.PreSharedKeys.PreSharedKeyFile) > 0 {
		cfg.PreSharedKeys = psk.NewConfig(o.PreSharedKeys)

	}

	return cfg
}

type completedConfig struct {
	AllowUnauthenticated bool

	Oidc          *oidc.CompletedConfig
	PreSharedKeys *psk.CompletedConfig
}

type CompletedConfig struct {
	*completedConfig
}

func (c *Config) Complete() (CompletedConfig, []error) {
	var errs []error
	cfg := CompletedConfig{&completedConfig{
		AllowUnauthenticated: c.AllowUnauthenticated,
	}}

	if c.Oidc != nil {
		if o, err := c.Oidc.Complete(); err == nil {
			cfg.Oidc = &o
		} else {
			errs = append(errs, err)
		}
	}

	if c.PreSharedKeys != nil {
		if o, err := c.PreSharedKeys.Complete(); err == nil {
			cfg.PreSharedKeys = &o
		} else {
			errs = append(errs, err)
		}
	}

	if errs != nil {
		return CompletedConfig{completedConfig: &completedConfig{}}, errs
	}

	return cfg, nil
}
