// Kratos v2.9.X's StreamMiddleware has a nuance
// where it counts metrics per-message instead of per-stream, causing "inflated"
// request counts for streaming RPCs. This provides a stream counter interceptor
// as a workaround, which uses native gRPC interceptors that correctly count once
// per stream establishment. It also provides per-message counting under a
// separate metric name via a wrapped ServerStream.
package grpc

import (
	"sync"
	"time"

	httpstatus "github.com/go-kratos/kratos/v2/transport/http/status"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	// stream connections (one increment per stream open).
	StreamCounterName = "grpc_server_streams_total"

	// individual messages sent and received across all streams.
	StreamMessageCounterName = "grpc_server_stream_messages_total"

	// time from stream open to first server response sent.
	StreamFirstResponseLatencyHistogramName = "grpc_server_stream_first_response_duration_seconds"

	// duration of individual send/receive operations.
	StreamMessageLatencyHistogramName = "grpc_server_stream_message_duration_seconds"
)

func NewStreamCounter(meter metric.Meter) (metric.Int64Counter, error) {
	return meter.Int64Counter(
		StreamCounterName,
		metric.WithDescription("Total number of gRPC stream connections"),
		metric.WithUnit("{stream}"),
	)
}

func NewStreamMessageCounter(meter metric.Meter) (metric.Int64Counter, error) {
	return meter.Int64Counter(
		StreamMessageCounterName,
		metric.WithDescription("Total number of gRPC stream messages sent and received"),
		metric.WithUnit("{message}"),
	)
}

func NewStreamFirstResponseLatencyHistogram(meter metric.Meter) (metric.Float64Histogram, error) {
	return meter.Float64Histogram(
		StreamFirstResponseLatencyHistogramName,
		metric.WithDescription("Time from stream open to first server response sent"),
		metric.WithUnit("s"),
	)
}

func NewStreamMessageLatencyHistogram(meter metric.Meter) (metric.Float64Histogram, error) {
	return meter.Float64Histogram(
		StreamMessageLatencyHistogramName,
		metric.WithDescription("Duration of individual stream message send/receive operations"),
		metric.WithUnit("s"),
	)
}

type monitoredStream struct {
	grpc.ServerStream
	messageCounter            metric.Int64Counter
	messageLatencyHist        metric.Float64Histogram
	firstResponseLatencyHist  metric.Float64Histogram
	attrs                     metric.MeasurementOption
	sentAttrs                 metric.MeasurementOption
	receivedAttrs             metric.MeasurementOption
	streamStart               time.Time
	firstResponseRecordedOnce sync.Once
}

func (s *monitoredStream) SendMsg(m interface{}) error {
	start := time.Now()
	err := s.ServerStream.SendMsg(m)
	if err == nil {
		if s.messageCounter != nil {
			s.messageCounter.Add(s.Context(), 1, s.attrs, s.sentAttrs)
		}
		if s.messageLatencyHist != nil {
			s.messageLatencyHist.Record(s.Context(), time.Since(start).Seconds(), s.attrs, s.sentAttrs)
		}
		if s.firstResponseLatencyHist != nil {
			s.firstResponseRecordedOnce.Do(func() {
				s.firstResponseLatencyHist.Record(s.Context(), time.Since(s.streamStart).Seconds(), s.attrs)
			})
		}
	}
	return err
}

func (s *monitoredStream) RecvMsg(m interface{}) error {
	start := time.Now()
	err := s.ServerStream.RecvMsg(m)
	if err == nil {
		if s.messageCounter != nil {
			s.messageCounter.Add(s.Context(), 1, s.attrs, s.receivedAttrs)
		}
		if s.messageLatencyHist != nil {
			s.messageLatencyHist.Record(s.Context(), time.Since(start).Seconds(), s.attrs, s.receivedAttrs)
		}
	}
	return err
}

type streamMetrics struct {
	counter                       metric.Int64Counter
	messageCounter                metric.Int64Counter
	firstResponseLatencyHistogram metric.Float64Histogram
	messageLatencyHistogram       metric.Float64Histogram
}

func newStreamMetrics(meter metric.Meter) (*streamMetrics, error) {
	if meter == nil {
		return nil, nil
	}
	counter, err := NewStreamCounter(meter)
	if err != nil {
		return nil, err
	}
	messageCounter, err := NewStreamMessageCounter(meter)
	if err != nil {
		return nil, err
	}
	firstResponseLatencyHistogram, err := NewStreamFirstResponseLatencyHistogram(meter)
	if err != nil {
		return nil, err
	}
	messageLatencyHistogram, err := NewStreamMessageLatencyHistogram(meter)
	if err != nil {
		return nil, err
	}
	return &streamMetrics{
		counter:                       counter,
		messageCounter:                messageCounter,
		firstResponseLatencyHistogram: firstResponseLatencyHistogram,
		messageLatencyHistogram:       messageLatencyHistogram,
	}, nil
}

func newStreamCounterInterceptor(m *streamMetrics) grpc.StreamServerInterceptor {
	return func(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		if m == nil {
			return handler(srv, ss)
		}

		streamAttrs := metric.WithAttributes(
			attribute.String("kind", "grpc"),
			attribute.String("operation", info.FullMethod),
		)

		wrapped := ss
		if m.messageCounter != nil || m.messageLatencyHistogram != nil || m.firstResponseLatencyHistogram != nil {
			wrapped = &monitoredStream{
				ServerStream:             ss,
				messageCounter:           m.messageCounter,
				messageLatencyHist:       m.messageLatencyHistogram,
				firstResponseLatencyHist: m.firstResponseLatencyHistogram,
				attrs:                    streamAttrs,
				sentAttrs:                metric.WithAttributes(attribute.String("direction", "sent")),
				receivedAttrs:            metric.WithAttributes(attribute.String("direction", "received")),
				streamStart:              time.Now(),
			}
		}

		err := handler(srv, wrapped)

		code := codes.OK
		if err != nil {
			if st, ok := status.FromError(err); ok {
				code = st.Code()
			} else {
				code = codes.Unknown
			}
		}

		codeAttrs := metric.WithAttributes(
			attribute.Int("code", httpstatus.FromGRPCCode(code)),
		)

		if m.counter != nil {
			m.counter.Add(ss.Context(), 1, streamAttrs, codeAttrs)
		}

		return err
	}
}
