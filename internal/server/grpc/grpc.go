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

// ServerConfig holds injectable dependencies for creating a gRPC server.
// This enables tests to inject their own implementations while sharing
// the same middleware construction logic as production.
type ServerConfig struct {
	Authenticator   authnapi.Authenticator
	AuthnMiddleware middleware.Middleware
	Metrics         middleware.Middleware
	Meter           metric.Meter
	Logger          log.Logger
	Validator       protovalidate.Validator
	ServerOptions   []kgrpc.ServerOption
	// GRPCOptions are raw grpc.ServerOption values (e.g. keepalive, max message
	// size) that NewWithDeps merges with interceptors into a single
	// kgrpc.Options() call.  Never wrap these in kgrpc.Options() yourself —
	// Kratos's Options() is a setter, not an appender.
	GRPCOptions  []grpc.ServerOption
	ReadOnlyMode bool
}

// New creates a new a gRPC server.
// authenticator is optional - if provided, uses the new aggregating authenticator for streams.
// If nil, falls back to OIDC-only authentication (backwards compatible).
func New(c CompletedConfig, authnMiddleware middleware.Middleware, authnConfig authn.CompletedConfig, authenticator authnapi.Authenticator, meter metric.Meter, logger log.Logger, readOnlyMode bool) (*kgrpc.Server, error) {
	requests, err := metrics.DefaultRequestsCounter(meter, metrics.DefaultServerRequestsCounterName)
	if err != nil {
		return nil, err
	}
	seconds, err := metrics.DefaultSecondsHistogram(meter, metrics.DefaultServerSecondsHistogramName)
	if err != nil {
		return nil, err
	}
	metricsMiddleware := metrics.Server(
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
	return NewWithDeps(ServerConfig{
		Authenticator:   authenticator,
		AuthnMiddleware: authnMiddleware,
		Metrics:         metricsMiddleware,
		Meter:           meter,
		Logger:          logger,
		Validator:       validator,
		ServerOptions:   c.ServerOptions,
		GRPCOptions:     c.GRPCOptions,
		ReadOnlyMode:    readOnlyMode,
	})
}

func NewWithDeps(deps ServerConfig) (*kgrpc.Server, error) {
	// Create stream metrics from the meter
	sm, err := newStreamMetrics(deps.Meter)
	if err != nil {
		return nil, err
	}

	// Stream counter is first so it captures all streams including auth failures
	streamingInterceptor := []grpc.StreamServerInterceptor{
		newStreamCounterInterceptor(sm),
		newStreamLoggingInterceptor(deps.Logger),
	}

	// Create stream interceptor using aggregating authenticator
	// If authenticator is nil, it will be created from config (backwards compatible)
	streamAuth, err := m.NewStreamAuthInterceptorFromAuthenticator(deps.Authenticator, deps.Logger)
	if err != nil {
		// If we can't create the authenticator, log warning but don't fail server startup
		// This maintains backwards compatibility for edge cases
		_ = deps.Logger.Log(log.LevelWarn, "msg", "Stream authentication interceptor not created", "error", err)
	} else {
		streamingInterceptor = append(streamingInterceptor, streamAuth.Interceptor())
	}

	streamingInterceptor = append(streamingInterceptor, m.StreamValidationInterceptor(deps.Validator))
	streamingInterceptor = append(streamingInterceptor, m.ErrorMappingStreamInterceptor())

	var authnMiddleware middleware.Middleware
	if deps.AuthnMiddleware != nil {
		authnMiddleware = deps.AuthnMiddleware
	} else {
		authnMiddleware = m.Authentication(deps.Authenticator)
	}

	// Build a single set of raw grpc.ServerOption that combines interceptors
	// with any options from the server config (e.g. keepalive policy).
	//
	// IMPORTANT: Kratos's kgrpc.Options() is a SETTER — each call replaces
	// s.grpcOpts rather than appending.  We must therefore produce exactly ONE
	// kgrpc.Options() call containing every raw grpc.ServerOption.  Splitting
	// them across multiple kgrpc.Options() calls causes only the last one to
	// survive, silently dropping interceptors or config.
	rawGRPCOpts := []grpc.ServerOption{
		grpc.ChainStreamInterceptor(streamingInterceptor...),
	}
	if deps.ReadOnlyMode {
		rawGRPCOpts = append(rawGRPCOpts, grpc.ChainUnaryInterceptor(m.UnaryReadOnlyInterceptor()))
	}
	rawGRPCOpts = append(rawGRPCOpts, deps.GRPCOptions...)

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
			// Logging intentionally omitted: Kratos StreamMiddleware runs middleware
			// per-message instead of per-stream, generating ~765 bytes of log per message
			// (including full proto args with continuation tokens). At 12M+ messages this
			// overwhelms the node log collector. Stream logging is handled by
			// newStreamLoggingInterceptor which logs once per stream open/close.
			//
			// Metrics intentionally omitted: Kratos StreamMiddleware counts per-message
			// instead of per-stream. Stream metrics are handled by newStreamCounterInterceptor.
		),
		kgrpc.Options(rawGRPCOpts...),
	}
	opts = append(opts, deps.ServerOptions...)
	srv := kgrpc.NewServer(opts...)
	return srv, nil
}
