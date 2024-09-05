package server

import (
	"context"

	"github.com/go-kratos/kratos/v2"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/middleware"
	kgrpc "github.com/go-kratos/kratos/v2/transport/grpc"
	khttp "github.com/go-kratos/kratos/v2/transport/http"

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

func New(c CompletedConfig, authn middleware.Middleware, logger log.Logger) (*Server, error) {
	s := &Server{
		Id:     c.Options.Id,
		Name:   c.Options.Name,
		Logger: log.With(logger, "service.id", c.Options.Id),
	}

	meterProvider, err := NewMeterProvider(s)
	if err != nil {
		return nil, err
	}

	meter, err := NewMeter(meterProvider)
	if err != nil {
		return nil, err
	}

	httpServer, err := http.New(c.HttpConfig, authn, meter)
	if err != nil {
		return nil, err
	}

	grpcServer, err := grpc.New(c.GrpcConfig, authn, meter)
	if err != nil {
		return nil, err
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
