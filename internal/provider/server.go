package provider

import (
	"fmt"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/middleware"

	authnapi "github.com/project-kessel/inventory-api/internal/authn/api"
	"github.com/project-kessel/inventory-api/internal/server"
	"github.com/project-kessel/inventory-api/internal/server/grpc"
	"github.com/project-kessel/inventory-api/internal/server/http"
	"github.com/project-kessel/inventory-api/internal/server/pprof"
)

// ServerConfig holds configuration for creating a Server.
type ServerConfig struct {
	Options         *ServerOptions
	AuthnMiddleware middleware.Middleware
	Authenticator   authnapi.Authenticator
	Logger          log.Logger
}

// NewServer creates a new Server from options and configuration.
func NewServer(cfg ServerConfig) (*server.Server, error) {
	if errs := cfg.Options.Validate(); len(errs) > 0 {
		return nil, fmt.Errorf("server validation failed: %v", errs)
	}

	// Complete pprof options (fill in defaults)
	cfg.Options.Pprof.Complete()

	// Convert provider options to server options
	serverOpts := &server.Options{
		Id:           cfg.Options.Id,
		Name:         cfg.Options.Name,
		PublicUrl:    cfg.Options.PublicUrl,
		GrpcOptions:  convertGrpcOptions(cfg.Options.Grpc),
		HttpOptions:  convertHttpOptions(cfg.Options.Http),
		PprofOptions: convertPprofOptions(cfg.Options.Pprof),
	}

	// Create server config and complete it
	serverConfig := server.NewConfig(serverOpts)
	completedServerConfig, errs := serverConfig.Complete()
	if len(errs) > 0 {
		return nil, fmt.Errorf("failed to complete server config: %v", errs)
	}

	return server.New(completedServerConfig, cfg.AuthnMiddleware, cfg.Authenticator, cfg.Logger)
}

// convertGrpcOptions converts provider GrpcOptions to grpc.Options.
func convertGrpcOptions(opts *GrpcOptions) *grpc.Options {
	return &grpc.Options{
		Addr:            opts.Addr,
		Timeout:         opts.Timeout,
		ServingCertFile: opts.ServingCertFile,
		PrivateKeyFile:  opts.PrivateKeyFile,
		ClientCAFile:    opts.ClientCAFile,
		SNI:             opts.SNI,
		CertOpt:         opts.CertOpt,
	}
}

// convertHttpOptions converts provider HttpOptions to http.Options.
func convertHttpOptions(opts *HttpOptions) *http.Options {
	return &http.Options{
		Addr:            opts.Addr,
		Timeout:         opts.Timeout,
		ServingCertFile: opts.ServingCertFile,
		PrivateKeyFile:  opts.PrivateKeyFile,
		ClientCAFile:    opts.ClientCAFile,
		SNI:             opts.SNI,
		CertOpt:         opts.CertOpt,
	}
}

// convertPprofOptions converts provider PprofOptions to pprof.Options.
func convertPprofOptions(opts *PprofOptions) *pprof.Options {
	return &pprof.Options{
		Enabled: opts.Enabled,
		Port:    opts.Port,
		Addr:    opts.Addr,
	}
}
