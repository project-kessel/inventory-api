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

type tokenClientConfig struct {
	clientId       string
	clientSecret   string
	url            string
	enableOIDCAuth bool
	insecure       bool
}

type completedConfig struct {
	gRPCConn    *grpc.ClientConn
	tokenConfig *tokenClientConfig
}

type CompletedConfig struct {
	*completedConfig
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

	tokenReq := &tokenClientConfig{
		clientId:       c.ClientId,
		clientSecret:   c.ClientSecret,
		url:            c.TokenEndpoint,
		enableOIDCAuth: c.EnableOidcAuth,
		insecure:       c.Insecure,
	}

	return CompletedConfig{&completedConfig{gRPCConn: conn, tokenConfig: tokenReq}}, nil
}
