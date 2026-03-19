package data

import (
	"context"

	"github.com/authzed/grpcutil"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// RelationsConfig holds the raw configuration for the relations repository.
type RelationsConfig struct {
	Impl    string
	SpiceDB *RelationsSpiceDBConfig
}

// RelationsSpiceDBConfig contains connection options for the SpiceDB Relations API.
type RelationsSpiceDBConfig struct {
	*RelationsOptions
}

// NewRelationsConfig creates a RelationsConfig from options.
func NewRelationsConfig(o *RelationsOptionsRoot) *RelationsConfig {
	var scfg *RelationsSpiceDBConfig
	if o.Impl == RelationsImplSpiceDB {
		scfg = &RelationsSpiceDBConfig{RelationsOptions: o.SpiceDB}
	}
	return &RelationsConfig{
		Impl:    o.Impl,
		SpiceDB: scfg,
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

	if c.Impl == RelationsImplSpiceDB {
		var opts []grpc.DialOption
		opts = append(opts, grpc.EmptyDialOption{})

		if c.SpiceDB.Insecure {
			opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
		} else {
			tlsConfig, _ := grpcutil.WithSystemCerts(grpcutil.VerifyCA)
			opts = append(opts, tlsConfig)
		}

		conn, err := grpc.NewClient(c.SpiceDB.URL, opts...)
		if err != nil {
			return RelationsCompletedConfig{}, []error{err}
		}

		cfg.gRPCConn = conn
		cfg.tokenConfig = &relationsTokenClientConfig{
			clientId:       c.SpiceDB.ClientId,
			clientSecret:   c.SpiceDB.ClientSecret,
			url:            c.SpiceDB.TokenEndpoint,
			enableOIDCAuth: c.SpiceDB.EnableOidcAuth,
			insecure:       c.SpiceDB.Insecure,
		}
	}

	return RelationsCompletedConfig{cfg}, nil
}
