package http

import (
	"buf.build/go/protovalidate"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/middleware"
	"github.com/go-kratos/kratos/v2/middleware/logging"
	"github.com/go-kratos/kratos/v2/middleware/metrics"
	"github.com/go-kratos/kratos/v2/middleware/recovery"
	"github.com/go-kratos/kratos/v2/middleware/selector"
	"github.com/go-kratos/kratos/v2/transport/http"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/otel/metric"

	authnapi "github.com/project-kessel/inventory-api/internal/authn/api"
	m "github.com/project-kessel/inventory-api/internal/middleware"
)

// ServerDeps holds injectable dependencies for creating an HTTP server.
// This enables tests to inject their own implementations while sharing
// the same middleware construction logic as production.
type ServerDeps struct {
	Authenticator authnapi.Authenticator
	Metrics       middleware.Middleware
	Logger        log.Logger
	Validator     protovalidate.Validator
	ServerOptions []http.ServerOption
}

// New creates a new HTTP server.
func New(c CompletedConfig, authn middleware.Middleware, meter metric.Meter, logger log.Logger, readOnlyMode bool) (*http.Server, error) {
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

	srv, err := NewWithDeps(ServerDeps{
		Metrics: metrics.Server(
			metrics.WithSeconds(seconds),
			metrics.WithRequests(requests),
		),
		Logger:        logger,
		Validator:     validator,
		ServerOptions: c.ServerOptions,
	}, authn, readOnlyMode)
	if err != nil {
		return nil, err
	}
	srv.HandlePrefix("/metrics", promhttp.HandlerFor(
		prometheus.DefaultGatherer,
		promhttp.HandlerOpts{
			EnableOpenMetrics: true,
		},
	))
	return srv, nil
}

// NewWithDeps creates an HTTP server from pre-built dependencies.
// If authnOverride is non-nil it is used as the authentication middleware;
// otherwise one is derived from deps.Authenticator.
func NewWithDeps(deps ServerDeps, authnOverride ...middleware.Middleware, readOnlyMode bool) (*http.Server, error) {
	var authnMiddleware middleware.Middleware
	if len(authnOverride) > 0 && authnOverride[0] != nil {
		authnMiddleware = authnOverride[0]
	} else {
		authnMiddleware = m.Authentication(deps.Authenticator)
	}

	// TODO: pass in health, authn middleware
	middlewares := []middleware.Middleware{
		recovery.Recovery(),
		logging.Server(logger),
		metrics.Server(
			metrics.WithSeconds(seconds),
			metrics.WithRequests(requests),
		),
		m.Validation(validator),
		selector.Server(
			authn,
		).Match(NewWhiteListMatcher).Build(),
		m.ErrorMapping(),
	}

	// Only add read-only middleware if in read-only mode to reduce overhead
	if readOnlyMode {
		middlewares = append(middlewares, m.HTTPReadOnlyMiddleware)
	}

	var opts = []http.ServerOption{
		http.Middleware(middlewares...),
	}
	opts = append(opts, c.ServerOptions...)
	srv := http.NewServer(opts...)
	return srv, nil
}
