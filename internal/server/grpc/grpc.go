package grpc

import (
	"github.com/bufbuild/protovalidate-go"
	"github.com/go-kratos/kratos/v2/middleware"
	"github.com/go-kratos/kratos/v2/middleware/recovery"
	"github.com/go-kratos/kratos/v2/middleware/selector"
	kgrpc "github.com/go-kratos/kratos/v2/transport/grpc"
	m "github.com/project-kessel/inventory-api/internal/middleware"
)

// New creates a new a gRPC server.
func New(c CompletedConfig, authn middleware.Middleware) *kgrpc.Server {
	validator, err := protovalidate.New()
	if err != nil {
		return nil
	}
	// TODO: pass in health, authn middleware
	var opts = []kgrpc.ServerOption{
		kgrpc.Middleware(
			recovery.Recovery(),
			m.Validation(validator),
			selector.Server(
				authn,
			).Match(NewWhiteListMatcher).Build(),
		),
	}
	opts = append(opts, c.ServerOptions...)
	srv := kgrpc.NewServer(opts...)
	return srv
}
