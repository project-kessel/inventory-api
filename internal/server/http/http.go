package http

import (
	"github.com/bufbuild/protovalidate-go"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/middleware"
	"github.com/go-kratos/kratos/v2/middleware/logging"
	"github.com/go-kratos/kratos/v2/middleware/metrics"
	"github.com/go-kratos/kratos/v2/middleware/recovery"
	"github.com/go-kratos/kratos/v2/middleware/selector"
	"github.com/go-kratos/kratos/v2/transport/http"
	m "github.com/project-kessel/inventory-api/internal/middleware"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/otel/metric"
)

// New create a new http server.
func New(c CompletedConfig, authn middleware.Middleware, meter metric.Meter, logger log.Logger) (*http.Server, error) {
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
	var opts = []http.ServerOption{
		http.Middleware(
			recovery.Recovery(),
			logging.Server(logger),
			m.Validation(validator),
			metrics.Server(
				metrics.WithSeconds(seconds),
				metrics.WithRequests(requests),
			),
			selector.Server(
				m.TransformMiddleware(),
				authn,
			).Match(NewWhiteListMatcher).Build(),
		),
	}
	opts = append(opts, c.ServerOptions...)
	srv := http.NewServer(opts...)
	srv.HandlePrefix("/metrics", promhttp.HandlerFor(
		prometheus.DefaultGatherer,
		promhttp.HandlerOpts{
			EnableOpenMetrics: true,
		},
	))
	return srv, nil
}
