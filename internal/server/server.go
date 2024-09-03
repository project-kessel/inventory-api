package server

import (
	"context"
	"go.opentelemetry.io/otel/metric"

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

func New(c CompletedConfig, authn middleware.Middleware, logger log.Logger, metrics metric.Meter) *Server {
	s := &Server{
		Id:         c.Options.Id,
		Name:       c.Options.Name,
		HttpServer: http.New(c.HttpConfig, authn, metrics),
		GrpcServer: grpc.New(c.GrpcConfig, authn, metrics),
		Logger:     log.With(logger, "service.id", c.Options.Id),
	}

	return s
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
