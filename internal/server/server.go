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

	"github.com/project-kessel/inventory-api/internal/server/grpc"
	"github.com/project-kessel/inventory-api/internal/server/http"
)

type Server struct {
	Id   string
	Name string

	HttpServer *khttp.Server
	GrpcServer *kgrpc.Server
	App        *kratos.App

	Logger log.Logger
}

func New(c CompletedConfig, authn middleware.Middleware, authnConfig authn.CompletedConfig, logger log.Logger) (*Server, error) {
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

	httpServer, err := http.New(c.HttpConfig, authn, meter, logger)
	if err != nil {
		return nil, fmt.Errorf("init http server failed: %w", err)
	}

	grpcServer, err := grpc.New(c.GrpcConfig, authn, authnConfig, meter, logger)
	if err != nil {
		return nil, fmt.Errorf("init grpc server failed: %w", err)
	}

	s.HttpServer = httpServer
	s.GrpcServer = grpcServer

	return s, nil
}

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

func (s *Server) Shutdown(ctx context.Context) error {
	return s.App.Stop()
}
