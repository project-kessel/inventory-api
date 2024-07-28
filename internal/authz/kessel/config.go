package kessel

import (
	"context"
	"crypto/tls"

	kratoshttp "github.com/go-kratos/kratos/v2/transport/http"
)

type Config struct {
	*Options
}

func NewConfig(o *Options) *Config {
	return &Config{Options: o}
}

type completedConfig struct {
	HttpClient *kratoshttp.Client
}

type CompletedConfig struct {
	*completedConfig
}

func (c *Config) Complete(ctx context.Context) (CompletedConfig, []error) {
	client, err := kratoshttp.NewClient(
		ctx,
		kratoshttp.WithEndpoint(c.URL),
		kratoshttp.WithTLSConfig(&tls.Config{InsecureSkipVerify: c.Insecure}),
	)
	if err != nil {
		return CompletedConfig{}, []error{err}
	}

	return CompletedConfig{&completedConfig{HttpClient: client}}, nil
}
