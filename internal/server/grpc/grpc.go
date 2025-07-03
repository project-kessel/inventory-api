package grpc

import (
	"fmt"

	"buf.build/go/protovalidate"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/middleware"
	"github.com/go-kratos/kratos/v2/middleware/logging"
	"github.com/go-kratos/kratos/v2/middleware/metrics"
	"github.com/go-kratos/kratos/v2/middleware/recovery"
	"github.com/go-kratos/kratos/v2/middleware/selector"
	kgrpc "github.com/go-kratos/kratos/v2/transport/grpc"
	"github.com/project-kessel/inventory-api/internal/authn"
	"github.com/project-kessel/inventory-api/internal/authn/interceptor"
	m "github.com/project-kessel/inventory-api/internal/middleware"
	"go.opentelemetry.io/otel/metric"
	"google.golang.org/grpc"
)

// New creates a new a gRPC server.
func New(c CompletedConfig, authn middleware.Middleware, authnConfig authn.CompletedConfig, meter metric.Meter, logger log.Logger) (*kgrpc.Server, error) {
	requests, err := metrics.DefaultRequestsCounter(meter, metrics.DefaultServerRequestsCounterName)
	if err != nil {
		return nil, err
	}
	seconds, err := metrics.DefaultSecondsHistogram(meter, metrics.DefaultServerSecondsHistogramName)
	if err != nil {
		return nil, err
	}
	validator, err := protovalidate.New()
	if err != nil {
		return nil, err
	}
	// TODO: pass in health, authn middleware
	var streamingInterceptor []grpc.StreamServerInterceptor
	if authnConfig.Oidc != nil {
		streamAuth, err := interceptor.NewStreamAuthInterceptor(authnConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to create stream auth interceptor: %w", err)
		}
		streamingInterceptor = []grpc.StreamServerInterceptor{
			streamAuth.Interceptor(),
		}
	}

	var opts = []kgrpc.ServerOption{
		kgrpc.Middleware(
			recovery.Recovery(),
			logging.Server(logger),
			m.Validation(validator),
			metrics.Server(
				metrics.WithRequests(requests),
				metrics.WithSeconds(seconds),
			),
			selector.Server(
				authn,
			).Match(NewWhiteListMatcher).Build(),
		),
		kgrpc.Options(grpc.ChainStreamInterceptor(
			streamingInterceptor...,
		)),
	}
	opts = append(opts, c.ServerOptions...)
	srv := kgrpc.NewServer(opts...)
	return srv, nil
}
