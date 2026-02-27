package grpc

import (
	"context"
	"testing"

	"net/http"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type mockServerStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (m *mockServerStream) Context() context.Context {
	return m.ctx
}

func (m *mockServerStream) SendMsg(interface{}) error {
	return nil
}

func (m *mockServerStream) RecvMsg(interface{}) error {
	return nil
}

func newTestStreamMetrics(t *testing.T, meter metric.Meter) *streamMetrics {
	sm, err := newStreamMetrics(meter)
	require.NoError(t, err)
	return sm
}

func TestStreamCounterInterceptor_CountsOncePerStream(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	meter := provider.Meter("test")

	sm := newTestStreamMetrics(t, meter)
	interceptor := newStreamCounterInterceptor(sm)

	stream := &mockServerStream{ctx: context.Background()}
	info := &grpc.StreamServerInfo{FullMethod: "/test.Service/StreamMethod"}

	callCount := 0
	handler := func(srv interface{}, ss grpc.ServerStream) error {
		callCount++
		return nil
	}

	// Call the interceptor multiple times (simulating multiple streams)
	for i := 0; i < 3; i++ {
		err := interceptor(nil, stream, info, handler)
		require.NoError(t, err)
	}

	assert.Equal(t, 3, callCount)

	// Verify metrics were recorded correctly
	var rm metricdata.ResourceMetrics
	err := reader.Collect(context.Background(), &rm)
	require.NoError(t, err)

	// Find the counter metric
	var counterSum int64
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			if m.Name == StreamCounterName {
				if sum, ok := m.Data.(metricdata.Sum[int64]); ok {
					for _, dp := range sum.DataPoints {
						counterSum += dp.Value
					}
				}
			}
		}
	}
	assert.Equal(t, int64(3), counterSum, "Stream counter should equal number of streams, not messages")
}

func TestStreamCounterInterceptor_NilCounter(t *testing.T) {
	interceptor := newStreamCounterInterceptor(nil)

	stream := &mockServerStream{ctx: context.Background()}
	info := &grpc.StreamServerInfo{FullMethod: "/test.Service/StreamMethod"}

	called := false
	handler := func(srv interface{}, ss grpc.ServerStream) error {
		called = true
		return nil
	}

	err := interceptor(nil, stream, info, handler)
	require.NoError(t, err)
	assert.True(t, called, "Handler should be called even with nil counter")
}

func TestStreamCounterInterceptor_GRPCErrorLabeling(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	meter := provider.Meter("test")

	sm := newTestStreamMetrics(t, meter)
	interceptor := newStreamCounterInterceptor(sm)

	stream := &mockServerStream{ctx: context.Background()}
	info := &grpc.StreamServerInfo{FullMethod: "/test.Service/StreamMethod"}

	handler := func(srv interface{}, ss grpc.ServerStream) error {
		return status.Error(codes.NotFound, "resource not found")
	}

	err := interceptor(nil, stream, info, handler)
	require.Error(t, err)

	var rm metricdata.ResourceMetrics
	err = reader.Collect(context.Background(), &rm)
	require.NoError(t, err)

	// Verify the error code was captured correctly
	metricFound := false
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			if m.Name == StreamCounterName {
				sum, ok := m.Data.(metricdata.Sum[int64])
				require.True(t, ok, "expected Sum[int64] data type")
				metricFound = true
				require.Len(t, sum.DataPoints, 1)
				attrs := sum.DataPoints[0].Attributes

				codeAttr, found := attrs.Value("code")
				require.True(t, found)
				assert.Equal(t, int64(http.StatusNotFound), codeAttr.AsInt64())

				kindAttr, found := attrs.Value("kind")
				require.True(t, found)
				assert.Equal(t, "grpc", kindAttr.AsString())

				opAttr, found := attrs.Value("operation")
				require.True(t, found)
				assert.Equal(t, "/test.Service/StreamMethod", opAttr.AsString())
			}
		}
	}
	require.True(t, metricFound, "expected %s metric to be emitted", StreamCounterName)
}

func TestStreamCounterInterceptor_MessageCounting(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	meter := provider.Meter("test")

	sm := newTestStreamMetrics(t, meter)
	interceptor := newStreamCounterInterceptor(sm)

	stream := &mockServerStream{ctx: context.Background()}
	info := &grpc.StreamServerInfo{FullMethod: "/test.Service/StreamMethod"}

	handler := func(srv interface{}, ss grpc.ServerStream) error {
		for i := 0; i < 5; i++ {
			if err := ss.SendMsg(nil); err != nil {
				return err
			}
		}
		if err := ss.RecvMsg(nil); err != nil {
			return err
		}
		return nil
	}

	err := interceptor(nil, stream, info, handler)
	require.NoError(t, err)

	var rm metricdata.ResourceMetrics
	err = reader.Collect(context.Background(), &rm)
	require.NoError(t, err)

	var streamCount int64
	var sentCount int64
	var receivedCount int64
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			if sum, ok := m.Data.(metricdata.Sum[int64]); ok {
				for _, dp := range sum.DataPoints {
					switch m.Name {
					case StreamCounterName:
						streamCount += dp.Value
					case StreamMessageCounterName:
						dir, found := dp.Attributes.Value("direction")
						require.True(t, found)
						switch dir.AsString() {
						case "sent":
							sentCount += dp.Value
						case "received":
							receivedCount += dp.Value
						}
					}
				}
			}
		}
	}

	assert.Equal(t, int64(1), streamCount, "Should count one stream")
	assert.Equal(t, int64(5), sentCount, "Should count 5 sent messages")
	assert.Equal(t, int64(1), receivedCount, "Should count 1 received message")
}

func TestStreamCounterInterceptor_NilMessageCounter(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	meter := provider.Meter("test")

	counter, err := NewStreamCounter(meter)
	require.NoError(t, err)

	interceptor := newStreamCounterInterceptor(&streamMetrics{counter: counter})

	stream := &mockServerStream{ctx: context.Background()}
	info := &grpc.StreamServerInfo{FullMethod: "/test.Service/StreamMethod"}

	handler := func(srv interface{}, ss grpc.ServerStream) error {
		return ss.SendMsg(nil)
	}

	err = interceptor(nil, stream, info, handler)
	require.NoError(t, err)

	// Should still count stream, just no message metrics
	var rm metricdata.ResourceMetrics
	err = reader.Collect(context.Background(), &rm)
	require.NoError(t, err)

	var streamCount int64
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			if m.Name == StreamCounterName {
				if sum, ok := m.Data.(metricdata.Sum[int64]); ok {
					for _, dp := range sum.DataPoints {
						streamCount += dp.Value
					}
				}
			}
		}
	}
	assert.Equal(t, int64(1), streamCount)
}

func TestStreamCounterInterceptor_FirstResponseLatencyHistogram(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	meter := provider.Meter("test")

	sm := newTestStreamMetrics(t, meter)
	interceptor := newStreamCounterInterceptor(sm)

	stream := &mockServerStream{ctx: context.Background()}
	info := &grpc.StreamServerInfo{FullMethod: "/test.Service/StreamMethod"}

	handler := func(srv interface{}, ss grpc.ServerStream) error {
		// Send multiple messages - only the first should be recorded
		for i := 0; i < 3; i++ {
			if err := ss.SendMsg(nil); err != nil {
				return err
			}
		}
		return nil
	}

	err := interceptor(nil, stream, info, handler)
	require.NoError(t, err)

	var rm metricdata.ResourceMetrics
	err = reader.Collect(context.Background(), &rm)
	require.NoError(t, err)

	metricFound := false
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			if m.Name == StreamFirstResponseLatencyHistogramName {
				hist, ok := m.Data.(metricdata.Histogram[float64])
				require.True(t, ok, "expected Histogram[float64] data type")
				metricFound = true
				require.Len(t, hist.DataPoints, 1)
				assert.Equal(t, uint64(1), hist.DataPoints[0].Count, "Should record exactly one observation despite multiple sends")
				assert.GreaterOrEqual(t, hist.DataPoints[0].Sum, 0.0)

				attrs := hist.DataPoints[0].Attributes
				kindAttr, found := attrs.Value("kind")
				require.True(t, found)
				assert.Equal(t, "grpc", kindAttr.AsString())
			}
		}
	}
	require.True(t, metricFound, "expected %s metric to be emitted", StreamFirstResponseLatencyHistogramName)
}

func TestStreamCounterInterceptor_MessageLatencyHistogram(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	meter := provider.Meter("test")

	sm := newTestStreamMetrics(t, meter)
	interceptor := newStreamCounterInterceptor(sm)

	stream := &mockServerStream{ctx: context.Background()}
	info := &grpc.StreamServerInfo{FullMethod: "/test.Service/StreamMethod"}

	handler := func(srv interface{}, ss grpc.ServerStream) error {
		for i := 0; i < 3; i++ {
			if err := ss.SendMsg(nil); err != nil {
				return err
			}
		}
		if err := ss.RecvMsg(nil); err != nil {
			return err
		}
		return nil
	}

	err := interceptor(nil, stream, info, handler)
	require.NoError(t, err)

	var rm metricdata.ResourceMetrics
	err = reader.Collect(context.Background(), &rm)
	require.NoError(t, err)

	metricFound := false
	var sentCount uint64
	var receivedCount uint64
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			if m.Name == StreamMessageLatencyHistogramName {
				hist, ok := m.Data.(metricdata.Histogram[float64])
				require.True(t, ok, "expected Histogram[float64] data type")
				metricFound = true
				for _, dp := range hist.DataPoints {
					dir, found := dp.Attributes.Value("direction")
					require.True(t, found)
					switch dir.AsString() {
					case "sent":
						sentCount += dp.Count
					case "received":
						receivedCount += dp.Count
					}
					assert.GreaterOrEqual(t, dp.Sum, 0.0)
				}
			}
		}
	}
	require.True(t, metricFound, "expected %s metric to be emitted", StreamMessageLatencyHistogramName)
	assert.Equal(t, uint64(3), sentCount, "Should have 3 sent latency observations")
	assert.Equal(t, uint64(1), receivedCount, "Should have 1 received latency observation")
}
