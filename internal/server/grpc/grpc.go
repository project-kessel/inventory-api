package grpc

import (
	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/middleware"
	"github.com/go-kratos/kratos/v2/middleware/metrics"
	"github.com/go-kratos/kratos/v2/middleware/recovery"
	"github.com/go-kratos/kratos/v2/middleware/selector"
	kgrpc "github.com/go-kratos/kratos/v2/transport/grpc"
	"go.opentelemetry.io/otel/metric"
)

// New creates a new a gRPC server.
func New(c CompletedConfig, authn middleware.Middleware, meter metric.Meter) *kgrpc.Server {

	requests, err := metrics.DefaultRequestsCounter(meter, metrics.DefaultServerRequestsCounterName)
	if err != nil {
		log.Errorf("Failed to set DefaultRequestCounter: %v", err)
	}
	seconds, err := metrics.DefaultSecondsHistogram(meter, metrics.DefaultServerSecondsHistogramName)
	if err != nil {
		log.Errorf("Failed to set DefaultSecondsHistogram: %v", err)
	}

	var opts = []kgrpc.ServerOption{
		kgrpc.Middleware(
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
	srv := kgrpc.NewServer(opts...)
	return srv
}
