package oidc

import (
	"net/http"

	"github.com/project-kessel/inventory-api/internal/authn/util"
)

type Config struct {
	*Options
	Client *http.Client
}

type completedConfig struct {
	*Config
}

type CompletedConfig struct {
	*completedConfig
}

func NewConfig(o *Options) *Config {
	return &Config{
		Options: o,
	}
}

func (c *Config) Complete() (CompletedConfig, error) {
	if c.Client == nil {
		c.Client = util.NewClient(c.InsecureClient)
	}
	return CompletedConfig{&completedConfig{c}}, nil
}
