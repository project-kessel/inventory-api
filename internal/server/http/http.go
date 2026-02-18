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

	m "github.com/project-kessel/inventory-api/internal/middleware"
)

// ServerConfig holds injectable dependencies for creating an HTTP server.
// This enables tests to inject their own implementations while sharing
// the same middleware construction logic as production.
type ServerConfig struct {
	AuthnMiddleware middleware.Middleware
	Metrics         middleware.Middleware
	Logger          log.Logger
	Validator       protovalidate.Validator
	ServerOptions   []http.ServerOption
	ReadOnlyMode    bool
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

	srv, err := NewWithDeps(ServerConfig{
		AuthnMiddleware: authn,
		Metrics: metrics.Server(
			metrics.WithSeconds(seconds),
			metrics.WithRequests(requests),
		),
		Logger:        logger,
		Validator:     validator,
		ServerOptions: c.ServerOptions,
		ReadOnlyMode:  readOnlyMode,
	})
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
func NewWithDeps(deps ServerConfig) (*http.Server, error) {
	// TODO: pass in health middleware
	middlewares := []middleware.Middleware{
		recovery.Recovery(),
		logging.Server(deps.Logger),
		deps.Metrics,
		m.Validation(deps.Validator),
		selector.Server(
			deps.AuthnMiddleware,
		).Match(NewWhiteListMatcher).Build(),
		m.ErrorMapping(),
	}

	// Only add read-only middleware if in read-only mode to reduce overhead
	if deps.ReadOnlyMode {
		middlewares = append(middlewares, m.HTTPReadOnlyMiddleware)
	}

	var opts = []http.ServerOption{
		http.Middleware(middlewares...),
	}
	opts = append(opts, deps.ServerOptions...)
	srv := http.NewServer(opts...)
	return srv, nil
}
