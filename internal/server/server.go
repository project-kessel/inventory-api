package server

import (
	"context"
	"fmt"

	"github.com/go-kratos/kratos/v2"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/middleware"
	kgrpc "github.com/go-kratos/kratos/v2/transport/grpc"
	khttp "github.com/go-kratos/kratos/v2/transport/http"
	"github.com/project-kessel/inventory-api/internal/authn"
	authnapi "github.com/project-kessel/inventory-api/internal/authn/api"

	"github.com/project-kessel/inventory-api/internal/server/grpc"
	"github.com/project-kessel/inventory-api/internal/server/http"
)

// Server represents the main application server containing both HTTP and gRPC servers.
type Server struct {
	Id   string
	Name string

	HttpServer *khttp.Server
	GrpcServer *kgrpc.Server
	App        *kratos.App

	Logger log.Logger
}

// New creates a new Server instance with the provided configuration and middleware.
// It initializes both HTTP and gRPC servers with the given authentication middleware.
// authenticator is optional - if provided, uses the new aggregating authenticator for streams.
// If nil, falls back to OIDC-only authentication (backwards compatible).
func New(c CompletedConfig, authn middleware.Middleware, metaAuthorizerMiddleware middleware.Middleware, authnConfig authn.CompletedConfig, authenticator authnapi.Authenticator, logger log.Logger) (*Server, error) {
	s := &Server{
		Id:     c.Options.Id,
		Name:   c.Options.Name,
		Logger: log.With(logger, "service.id", c.Options.Id),
	}

	meterProvider, err := NewMeterProvider(s)
	if err != nil {
		return nil, fmt.Errorf("init meter provider failed: %w", err)
	}

	meter, err := NewMeter(meterProvider)
	if err != nil {
		return nil, fmt.Errorf("init meter failed: %w", err)
	}

	httpServer, err := http.New(c.HttpConfig, authn, metaAuthorizerMiddleware, meter, logger)
	if err != nil {
		return nil, fmt.Errorf("init http server failed: %w", err)
	}

	// Pass authenticator to gRPC server for stream interceptor
	// If nil, the interceptor will fall back to OIDC-only authentication (backwards compatible)
	grpcServer, err := grpc.New(c.GrpcConfig, authn, metaAuthorizerMiddleware, authnConfig, authenticator, meter, logger)
	if err != nil {
		return nil, fmt.Errorf("init grpc server failed: %w", err)
	}

	s.HttpServer = httpServer
	s.GrpcServer = grpcServer

	return s, nil
}

// Run starts the server and blocks until the context is cancelled or an error occurs.
func (s *Server) Run(ctx context.Context) error {
	s.App = kratos.New(
		kratos.ID(s.Id),
		kratos.Name(s.Name),
		kratos.Logger(s.Logger),
		kratos.Metadata(map[string]string{}),
		kratos.Context(ctx),
		kratos.Server(s.GrpcServer, s.HttpServer),
	)
	return s.App.Run()
}

// Shutdown gracefully shuts down the server.
func (s *Server) Shutdown(ctx context.Context) error {
	return s.App.Stop()
}
