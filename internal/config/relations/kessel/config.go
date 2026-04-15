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
	clientId       string
	clientSecret   string
	url            string
	enableOIDCAuth bool
	insecure       bool
}

func (t *TokenClientConfig) GetClientId() string     { return t.clientId }
func (t *TokenClientConfig) GetClientSecret() string { return t.clientSecret }
func (t *TokenClientConfig) GetURL() string          { return t.url }
func (t *TokenClientConfig) GetEnableOIDCAuth() bool { return t.enableOIDCAuth }
func (t *TokenClientConfig) GetInsecure() bool       { return t.insecure }

type completedConfig struct {
	grpcConn    *grpc.ClientConn
	tokenConfig *TokenClientConfig
}

type CompletedConfig struct {
	*completedConfig
}

func (c CompletedConfig) GetGRPCConn() *grpc.ClientConn {
	return c.grpcConn
}

func (c CompletedConfig) GetTokenConfig() *TokenClientConfig {
	return c.tokenConfig
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
		clientId:       c.ClientId,
		clientSecret:   c.ClientSecret,
		url:            c.TokenEndpoint,
		enableOIDCAuth: c.EnableOidcAuth,
		insecure:       c.Insecure,
	}

	return CompletedConfig{&completedConfig{grpcConn: conn, tokenConfig: tokenReq}}, nil
}
