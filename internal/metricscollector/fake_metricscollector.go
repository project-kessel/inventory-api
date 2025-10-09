package metricscollector

import (
	"context"
	"sync"

	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/metric/embedded"
)

type fakeMetricsState struct {
	mu sync.Mutex

	SerializationFailureCount    int
	SerializationExhaustionCount int
	OutboxEventWriteCount        int
	MsgProcessedCount            int
	MsgProcessFailureCount       int
	ConsumerErrorCount           int
	KafkaErrorEventCount         int
}

var globalFakeState = &fakeMetricsState{}

func NewFakeMetricsCollector() *MetricsCollector {
	globalFakeState.Reset()
	mc := &MetricsCollector{
		SerializationFailures:    &fakeCounter{counterType: "serialization_failures"},
		SerializationExhaustions: &fakeCounter{counterType: "serialization_exhaustions"},
		OutboxEventWrites:        &fakeCounter{counterType: "outbox_event_writes"},
		MsgsProcessed:            &fakeCounter{counterType: "msgs_processed"},
		MsgProcessFailures:       &fakeCounter{counterType: "msg_process_failures"},
		ConsumerErrors:           &fakeCounter{counterType: "consumer_errors"},
		KafkaErrorEvents:         &fakeCounter{counterType: "kafka_error_events"},
	}
	return mc
}

func (s *fakeMetricsState) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.SerializationFailureCount = 0
	s.SerializationExhaustionCount = 0
	s.OutboxEventWriteCount = 0
	s.MsgProcessedCount = 0
	s.MsgProcessFailureCount = 0
	s.ConsumerErrorCount = 0
	s.KafkaErrorEventCount = 0
}

func GetSerializationFailureCount() int {
	globalFakeState.mu.Lock()
	defer globalFakeState.mu.Unlock()
	return globalFakeState.SerializationFailureCount
}

func GetSerializationExhaustionCount() int {
	globalFakeState.mu.Lock()
	defer globalFakeState.mu.Unlock()
	return globalFakeState.SerializationExhaustionCount
}

func GetOutboxEventWriteCount() int {
	globalFakeState.mu.Lock()
	defer globalFakeState.mu.Unlock()
	return globalFakeState.OutboxEventWriteCount
}

func incrementCounter(counterType string) {
	globalFakeState.mu.Lock()
	defer globalFakeState.mu.Unlock()
	switch counterType {
	case "serialization_failures":
		globalFakeState.SerializationFailureCount++
	case "serialization_exhaustions":
		globalFakeState.SerializationExhaustionCount++
	case "outbox_event_writes":
		globalFakeState.OutboxEventWriteCount++
	case "msgs_processed":
		globalFakeState.MsgProcessedCount++
	case "msg_process_failures":
		globalFakeState.MsgProcessFailureCount++
	case "consumer_errors":
		globalFakeState.ConsumerErrorCount++
	case "kafka_error_events":
		globalFakeState.KafkaErrorEventCount++
	}
}

type fakeCounter struct {
	embedded.Int64Counter
	counterType string
}

func (fc *fakeCounter) Add(ctx context.Context, incr int64, options ...metric.AddOption) {
	incrementCounter(fc.counterType)
}

var _ metric.Int64Counter = (*fakeCounter)(nil)
