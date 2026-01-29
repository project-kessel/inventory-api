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
	m "github.com/project-kessel/inventory-api/internal/middleware"
	"go.opentelemetry.io/otel/metric"
	"google.golang.org/grpc"
)

// ServerDeps holds injectable dependencies for creating a gRPC server.
// This enables tests to inject their own implementations while sharing
// the same middleware construction logic as production.
type ServerDeps struct {
	Authenticator   authnapi.Authenticator
	AuthnMiddleware middleware.Middleware
	Metrics         middleware.Middleware
	Logger          log.Logger
	Validator       protovalidate.Validator
	ServerOptions   []kgrpc.ServerOption
}

// New creates a new a gRPC server.
// authenticator is optional - if provided, uses the new aggregating authenticator for streams.
// If nil, falls back to OIDC-only authentication (backwards compatible).
func New(c CompletedConfig, authnMiddleware middleware.Middleware, authnConfig authn.CompletedConfig, authenticator authnapi.Authenticator, meter metric.Meter, logger log.Logger) (*kgrpc.Server, error) {
	requests, err := metrics.DefaultRequestsCounter(meter, metrics.DefaultServerRequestsCounterName)
	if err != nil {
		return nil, err
	}
	seconds, err := metrics.DefaultSecondsHistogram(meter, metrics.DefaultServerSecondsHistogramName)
	if err != nil {
		return nil, err
	}
	metrics := metrics.Server(
		metrics.WithRequests(requests),
		metrics.WithSeconds(seconds),
	)
	validator, err := protovalidate.New()
	if err != nil {
		return nil, err
	}
	authnLogger := log.NewHelper(log.With(logger, "subsystem", "authn", "component", "stream-interceptor"))
	if authenticator == nil {
		authenticator, err = authn.New(authnConfig, authnLogger)
		if err != nil {
			return nil, err
		}
	}
	return NewWithDeps(ServerDeps{
		Authenticator:   authenticator,
		AuthnMiddleware: authnMiddleware,
		Metrics:         metrics,
		Logger:          logger,
		Validator:       validator,
		ServerOptions:   c.ServerOptions,
	})
}

func NewWithDeps(deps ServerDeps) (*kgrpc.Server, error) {
	// TODO: pass in health, authn middleware
	// Error mapping interceptor is always added for streaming RPCs
	streamingInterceptor := []grpc.StreamServerInterceptor{
		m.ErrorMappingStreamInterceptor(),
	}

	// Create stream interceptor using aggregating authenticator
	// If authenticator is nil, it will be created from config (backwards compatible)
	streamAuth, err := m.NewStreamAuthInterceptorFromAuthenticator(deps.Authenticator, deps.Logger)
	if err != nil {
		// If we can't create the authenticator, log warning but don't fail server startup
		// This maintains backwards compatibility for edge cases
		_ = deps.Logger.Log(log.LevelWarn, "msg", "Stream authentication interceptor not created", "error", err)
	} else {
		// Auth interceptor runs before error mapping (added to front of chain)
		streamingInterceptor = append([]grpc.StreamServerInterceptor{streamAuth.Interceptor()}, streamingInterceptor...)
	}

	var authnMiddleware middleware.Middleware
	if deps.AuthnMiddleware != nil {
		authnMiddleware = deps.AuthnMiddleware
	} else {
		authnMiddleware = m.Authentication(deps.Authenticator)
	}

	var opts = []kgrpc.ServerOption{
		kgrpc.Middleware(
			recovery.Recovery(),
			logging.Server(deps.Logger),
			deps.Metrics,
			m.Validation(deps.Validator),
			selector.Server(
				authnMiddleware,
			).Match(NewWhiteListMatcher).Build(),
			m.ErrorMapping(),
		),
		kgrpc.StreamMiddleware(
			recovery.Recovery(),
			logging.Server(deps.Logger),
			deps.Metrics,
			// Error mapping handled by interceptor due to Kratos limitations
		),
		kgrpc.Options(grpc.ChainStreamInterceptor(
			streamingInterceptor...,
		)),
	}
	opts = append(opts, deps.ServerOptions...)
	srv := kgrpc.NewServer(opts...)
	return srv, nil
}
