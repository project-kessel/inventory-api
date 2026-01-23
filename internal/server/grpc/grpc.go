package grpc

import (
	"buf.build/go/protovalidate"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/middleware"
	"github.com/go-kratos/kratos/v2/middleware/logging"
	"github.com/go-kratos/kratos/v2/middleware/metrics"
	"github.com/go-kratos/kratos/v2/middleware/recovery"
	"github.com/go-kratos/kratos/v2/middleware/selector"
	kgrpc "github.com/go-kratos/kratos/v2/transport/grpc"
	"github.com/project-kessel/inventory-api/internal/authn"
	authnapi "github.com/project-kessel/inventory-api/internal/authn/api"
	"github.com/project-kessel/inventory-api/internal/authn/interceptor"
	m "github.com/project-kessel/inventory-api/internal/middleware"
	"go.opentelemetry.io/otel/metric"
	"google.golang.org/grpc"
)

// New creates a new a gRPC server.
// authenticator is optional - if provided, uses the new aggregating authenticator for streams.
// If nil, falls back to OIDC-only authentication (backwards compatible).
func New(c CompletedConfig, authn middleware.Middleware, authnConfig authn.CompletedConfig, authenticator authnapi.Authenticator, meter metric.Meter, logger log.Logger) (*kgrpc.Server, error) {
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

	// Create stream interceptor using aggregating authenticator
	// If authenticator is nil, it will be created from config (backwards compatible)
	streamAuth, err := interceptor.NewStreamAuthInterceptor(authnConfig, authenticator, logger)
	if err != nil {
		// If we can't create the authenticator, log warning but don't fail server startup
		// This maintains backwards compatibility for edge cases
		_ = logger.Log(log.LevelWarn, "msg", "Stream authentication interceptor not created", "error", err)
	} else {
		streamingInterceptor = []grpc.StreamServerInterceptor{
			streamAuth.Interceptor(),
		}
	}

	var opts = []kgrpc.ServerOption{
		kgrpc.Middleware(
			recovery.Recovery(),
			logging.Server(logger),
			metrics.Server(
				metrics.WithRequests(requests),
				metrics.WithSeconds(seconds),
			),
			m.Validation(validator),
			selector.Server(
				authn,
			).Match(NewWhiteListMatcher).Build(),
		),
		kgrpc.StreamMiddleware(
			recovery.Recovery(),
			logging.Server(logger),
			metrics.Server(
				metrics.WithRequests(requests),
				metrics.WithSeconds(seconds),
			),
		),
		kgrpc.Options(grpc.ChainStreamInterceptor(
			streamingInterceptor...,
		)),
	}
	opts = append(opts, c.ServerOptions...)
	srv := kgrpc.NewServer(opts...)
	return srv, nil
}
