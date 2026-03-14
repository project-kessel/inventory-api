package data

import (
	"context"

	"github.com/authzed/grpcutil"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// RelationsConfig holds the raw configuration for the relations repository.
type RelationsConfig struct {
	Impl   string
	Kessel *RelationsKesselConfig
}

// RelationsKesselConfig contains connection options for the Kessel Relations API.
type RelationsKesselConfig struct {
	*RelationsOptions
}

// NewRelationsConfig creates a RelationsConfig from options.
func NewRelationsConfig(o *RelationsOptionsRoot) *RelationsConfig {
	var kcfg *RelationsKesselConfig
	if o.Impl == RelationsImplKessel {
		kcfg = &RelationsKesselConfig{RelationsOptions: o.Kessel}
	}
	return &RelationsConfig{
		Impl:   o.Impl,
		Kessel: kcfg,
	}
}

type relationsTokenClientConfig struct {
	clientId       string
	clientSecret   string
	url            string
	enableOIDCAuth bool
	insecure       bool
}

type relationsCompletedConfig struct {
	Impl        string
	gRPCConn    *grpc.ClientConn
	tokenConfig *relationsTokenClientConfig
}

// RelationsCompletedConfig is the validated/completed relations configuration.
type RelationsCompletedConfig struct {
	*relationsCompletedConfig
}

// Complete validates and finalizes the RelationsConfig, establishing gRPC connections as needed.
func (c *RelationsConfig) Complete(ctx context.Context) (RelationsCompletedConfig, []error) {
	cfg := &relationsCompletedConfig{
		Impl: c.Impl,
	}

	if c.Impl == RelationsImplKessel {
		var opts []grpc.DialOption
		opts = append(opts, grpc.EmptyDialOption{})

		if c.Kessel.Insecure {
			opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
		} else {
			tlsConfig, _ := grpcutil.WithSystemCerts(grpcutil.VerifyCA)
			opts = append(opts, tlsConfig)
		}

		conn, err := grpc.NewClient(c.Kessel.URL, opts...)
		if err != nil {
			return RelationsCompletedConfig{}, []error{err}
		}

		cfg.gRPCConn = conn
		cfg.tokenConfig = &relationsTokenClientConfig{
			clientId:       c.Kessel.ClientId,
			clientSecret:   c.Kessel.ClientSecret,
			url:            c.Kessel.TokenEndpoint,
			enableOIDCAuth: c.Kessel.EnableOidcAuth,
			insecure:       c.Kessel.Insecure,
		}
	}

	return RelationsCompletedConfig{cfg}, nil
}
