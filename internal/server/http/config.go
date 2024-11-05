package http

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/go-kratos/kratos/v2/transport/http"
)

type Config struct {
	Options   *Options
	TLSConfig *tls.Config
}

type completedConfig struct {
	Options       *Options
	ServerOptions []http.ServerOption
}

// CompletedConfig can be constructed only from Config.Complete
type CompletedConfig struct {
	*completedConfig
}

func NewConfig(o *Options) *Config {
	return &Config{
		Options:   o,
		TLSConfig: nil,
	}
}

// TODO: Update the server so the serving cert and client CA are rotated without downtime (see: hitless rotation).
func (c *Config) getTSLConfig() (*tls.Config, error) {
	if c.TLSConfig != nil {
		return c.TLSConfig, nil
	}

	if c.Options.ServingCertFile != "" && c.Options.PrivateKeyFile != "" {
		config := &tls.Config{}

		var err error
		config.Certificates = make([]tls.Certificate, 1)
		if config.Certificates[0], err = tls.LoadX509KeyPair(c.Options.ServingCertFile, c.Options.PrivateKeyFile); err != nil {
			return nil, fmt.Errorf("failed to load X509 key pair: %w", err)
		}

		if c.Options.CertOpt > int(tls.NoClientCert) && c.Options.ClientCAFile != "" {
			var caCertPool *x509.CertPool
			if file, err := os.Open(c.Options.ClientCAFile); err == nil {
				if caCert, err := io.ReadAll(file); err == nil {
					caCertPool = x509.NewCertPool()
					caCertPool.AppendCertsFromPEM(caCert)
				} else {
					return nil, fmt.Errorf("failed to load CA certificate: %w", err)
				}
			}

			config.ServerName = c.Options.SNI
			config.ClientAuth = tls.ClientAuthType(c.Options.CertOpt)
			config.ClientCAs = caCertPool
			config.MinVersion = tls.VersionTLS12
		}
		return config, nil
	}

	return nil, nil
}

func (c *Config) Complete() (CompletedConfig, error) {
	var tlsConfig *tls.Config
	if t, err := c.getTSLConfig(); err != nil {
		return CompletedConfig{}, err
	} else {
		tlsConfig = t
	}
	c.TLSConfig = tlsConfig
	opts := []http.ServerOption{
		http.Address(c.Options.Addr),
		http.TLSConfig(tlsConfig),
		http.StrictSlash(true),
		http.Timeout(time.Duration(c.Options.Timeout) * time.Second),
	}

	return CompletedConfig{&completedConfig{
		Options:       c.Options,
		ServerOptions: opts,
	}}, nil
}

func NewWhiteListMatcher(ctx context.Context, operation string) bool {
	whiteList := make(map[string]struct{})
	whiteList["/kessel.inventory.v1.KesselInventoryHealthService/GetReadyz"] = struct{}{}
	whiteList["/kessel.inventory.v1.KesselInventoryHealthService/GetLivez"] = struct{}{}
	whiteList["/grpc.health.v1.Health/Check"] = struct{}{}
	if _, ok := whiteList[operation]; ok {
		return false
	}
	return true
}
