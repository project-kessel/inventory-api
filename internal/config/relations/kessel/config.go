package kessel

import (
	"context"

	"github.com/authzed/grpcutil"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type Config struct {
	*Options
}

func NewConfig(o *Options) *Config {
	return &Config{Options: o}
}

type TokenClientConfig struct {
	ClientId       string
	ClientSecret   string
	URL            string
	EnableOIDCAuth bool
	Insecure       bool
}

type completedConfig struct {
	GRPCConn    *grpc.ClientConn
	TokenConfig *TokenClientConfig
}

type CompletedConfig struct {
	*completedConfig
}

func (c CompletedConfig) GetGRPCConn() *grpc.ClientConn {
	return c.GRPCConn
}

func (c CompletedConfig) GetTokenConfig() *TokenClientConfig {
	return c.TokenConfig
}

func (c *Config) Complete(ctx context.Context) (CompletedConfig, []error) {
	var opts []grpc.DialOption
	opts = append(opts, grpc.EmptyDialOption{})

	if c.Insecure {
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	} else {
		tlsConfig, _ := grpcutil.WithSystemCerts(grpcutil.VerifyCA)
		opts = append(opts, tlsConfig)
	}

	conn, err := grpc.NewClient(
		c.URL,
		opts...,
	)
	if err != nil {
		return CompletedConfig{}, []error{err}
	}

	tokenReq := &TokenClientConfig{
		ClientId:       c.ClientId,
		ClientSecret:   c.ClientSecret,
		URL:            c.TokenEndpoint,
		EnableOIDCAuth: c.EnableOidcAuth,
		Insecure:       c.Insecure,
	}

	return CompletedConfig{&completedConfig{GRPCConn: conn, TokenConfig: tokenReq}}, nil
}
