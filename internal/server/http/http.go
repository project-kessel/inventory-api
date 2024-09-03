package http

import (
	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/middleware"
	"github.com/go-kratos/kratos/v2/middleware/metrics"
	"github.com/go-kratos/kratos/v2/middleware/recovery"
	"github.com/go-kratos/kratos/v2/middleware/selector"
	"github.com/go-kratos/kratos/v2/transport/http"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/otel/metric"
)

// New create a new http server.
func New(c CompletedConfig, authn middleware.Middleware, meter metric.Meter) *http.Server {
	requests, err := metrics.DefaultRequestsCounter(meter, metrics.DefaultServerRequestsCounterName)
	if err != nil {
		log.Errorf("Failed to set DefaultRequestCounter: %v", err)
	}
	seconds, err := metrics.DefaultSecondsHistogram(meter, metrics.DefaultServerSecondsHistogramName)
	if err != nil {
		log.Errorf("Failed to set DefaultSecondsHistogram: %v", err)
	}

	var opts = []http.ServerOption{
		http.Middleware(
			recovery.Recovery(),
			metrics.Server(
				metrics.WithSeconds(seconds),
				metrics.WithRequests(requests),
			),
			selector.Server(
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
	return srv
}
