package server

import (
	"github.com/project-kessel/inventory-api/internal/server/grpc"
	"github.com/project-kessel/inventory-api/internal/server/http"
)

type Config struct {
	Options *Options

	GrpcConfig *grpc.Config
	HttpConfig *http.Config
}

type completedConfig struct {
	Options *Options

	GrpcConfig grpc.CompletedConfig
	HttpConfig http.CompletedConfig
}

type CompletedConfig struct {
	*completedConfig
}

func NewConfig(o *Options) *Config {
	return &Config{
		Options: o,

		GrpcConfig: grpc.NewConfig(o.GrpcOptions),
		HttpConfig: http.NewConfig(o.HttpOptions),
	}
}

func (c *Config) Complete() (CompletedConfig, []error) {
	var errs []error
	grpcConfig, err := c.GrpcConfig.Complete()
	if err != nil {
		errs = append(errs, err)
	}

	httpConfig, err := c.HttpConfig.Complete()
	if err != nil {
		errs = append(errs, err)
	}

	return CompletedConfig{&completedConfig{
		Options: c.Options,

		GrpcConfig: grpcConfig,
		HttpConfig: httpConfig,
	}}, errs
}
