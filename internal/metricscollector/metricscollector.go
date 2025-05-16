package metricscollector

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

const (
	outboxTopic = "outbox.event.kessel.tuples"
	// consumerPrefix should be used for all consumer-related metrics
	consumerPrefix = "kessel_inventory_consumer_"
	// prefix should be used for all other metrics unrelated to the consumer
	prefix = "kessel_inventory_"
)

// LabelSet adds desired attributes to each metric recorded from stats messages to ensure consistent labeling.
func (s *StatsData) LabelSet(key string) metric.MeasurementOption {
	if key == "" {
		m := metric.WithAttributes(
			attribute.String("name", s.Name),
			attribute.String("client_id", s.ClientID))
		return m
	}
	m := metric.WithAttributes(
		attribute.String("name", s.Name),
		attribute.String("client_id", s.ClientID),
		attribute.String("topic", outboxTopic),
		attribute.String("partition", key))
	return m
}

// StatsData defines the key metrics to be monitored provided by a kafka.Stats message. It contains top-level metrics and objects within the message.
type StatsData struct {
	Name     string               `json:"name"`
	ClientID string               `json:"client_id"`
	Replyq   int64                `json:"replyq"`
	Topics   map[string]TopicData `json:"topics"`
	CGRP     CGRPData             `json:"cgrp"`
}

// TopicData contains metrics from the 'topic' section of a stats message
type TopicData struct {
	Topic      string                   `json:"topic"`
	Partitions map[string]PartitionData `json:"partitions"`
}

// PartitionData contains metrics from the 'partitions' array section of a stats message
type PartitionData struct {
	FetchqCnt         int64  `json:"fetchq_cnt"`
	FetchqSize        int64  `json:"fetchq_size"`
	FetchState        string `json:"fetch_state"`
	LoOffset          int64  `json:"lo_offset"`
	HiOffset          int64  `json:"hi_offset"`
	LsOffset          int64  `json:"ls_offset"`
	ConsumerLag       int64  `json:"consumer_lag"`
	ConsumerLagStored int64  `json:"consumer_lag_stored"`
}

// CGRPData contains metrics from the 'cgrp' array section of a stats message. It captures metrics on the consumer group.
type CGRPData struct {
	State           string `json:"state"`
	StateAge        int64  `json:"stageage"`
	RebalanceAge    int64  `json:"rebalance_age"`
	RebalanceCnt    int64  `json:"rebalance_cnt"`
	RebalanceReason string `json:"rebalance_reason"`
	AssignmentSize  int64  `json:"assignment_size"`
}

// MetricsCollector captures metrics from stats messages and from custom app-centric messages
type MetricsCollector struct {
	// Top-Level Metrics
	replyq metric.Int64Gauge

	// Topic.Partitions Metrics
	fetchqCnt         metric.Int64Gauge
	fetchqSize        metric.Int64Gauge
	fetchState        metric.Int64Gauge
	loOffset          metric.Int64Gauge
	hiOffset          metric.Int64Gauge
	lsOffset          metric.Int64Gauge
	consumerLag       metric.Int64Gauge
	consumerLagStored metric.Int64Gauge

	// CGRP Metrics
	state          metric.Int64Gauge
	stateAge       metric.Int64Gauge
	rebalanceAge   metric.Int64Gauge
	rebalanceCnt   metric.Int64Counter
	assignmentSize metric.Int64Gauge

	// App Specific Metrics
	MsgsProcessed      metric.Int64Counter
	MsgProcessFailures metric.Int64Counter
	ConsumerErrors     metric.Int64Counter
	KafkaErrorEvents   metric.Int64Counter
	OutboxEventWrites  metric.Int64Counter
}

// New instantiates a new MetricsCollector
func (m *MetricsCollector) New(meter metric.Meter) error {
	var err error

	// create top-level metrics
	if m.replyq, err = meter.Int64Gauge(consumerPrefix + "replyq"); err != nil {
		return err
	}

	// create topic.partitions metrics
	if m.fetchqCnt, err = meter.Int64Gauge(consumerPrefix + "fetchq_cnt"); err != nil {
		return err
	}
	if m.fetchqSize, err = meter.Int64Gauge(consumerPrefix + "fetchq_size"); err != nil {
		return err
	}
	if m.fetchState, err = meter.Int64Gauge(consumerPrefix + "fetchq_state"); err != nil {
		return err
	}
	if m.loOffset, err = meter.Int64Gauge(consumerPrefix + "lo_offset"); err != nil {
		return err
	}
	if m.hiOffset, err = meter.Int64Gauge(consumerPrefix + "hi_offset"); err != nil {
		return err
	}
	if m.lsOffset, err = meter.Int64Gauge(consumerPrefix + "ls_offset"); err != nil {
		return err
	}
	if m.consumerLag, err = meter.Int64Gauge(consumerPrefix + "consumer_lag"); err != nil {
		return err
	}
	if m.consumerLagStored, err = meter.Int64Gauge(consumerPrefix + "consumer_lag_stored"); err != nil {
		return err
	}

	// create cgrp metrics
	if m.state, err = meter.Int64Gauge(consumerPrefix + "state"); err != nil {
		return err
	}
	if m.stateAge, err = meter.Int64Gauge(consumerPrefix + "stateage"); err != nil {
		return err
	}
	if m.rebalanceAge, err = meter.Int64Gauge(consumerPrefix + "rebalance_age"); err != nil {
		return err
	}
	if m.rebalanceCnt, err = meter.Int64Counter(consumerPrefix + "rebalance_cnt"); err != nil {
		return err
	}
	if m.assignmentSize, err = meter.Int64Gauge(consumerPrefix + "assignment_size"); err != nil {
		return err
	}

	// create consumer custom app metrics
	if m.MsgsProcessed, err = meter.Int64Counter(consumerPrefix + "msgs_processed"); err != nil {
		return err
	}
	if m.MsgProcessFailures, err = meter.Int64Counter(consumerPrefix + "msg_process_failures"); err != nil {
		return err
	}
	if m.ConsumerErrors, err = meter.Int64Counter(consumerPrefix + "consumer_errors"); err != nil {
		return err
	}
	if m.KafkaErrorEvents, err = meter.Int64Counter(consumerPrefix + "kafka_error_events"); err != nil {
		return err
	}

	// create all other custom app metrics
	if m.OutboxEventWrites, err = meter.Int64Counter(prefix + "outbox_event_writes"); err != nil {
		return err
	}
	return nil
}

// Collect is called on every stats message received to scrape the metrics and report them in our metrics endpoint
func (m *MetricsCollector) Collect(stats StatsData) {
	// top-level
	ctx := context.Background()
	m.replyq.Record(ctx, stats.Replyq, stats.LabelSet(""))

	// topics.partitions
	for partitionKey := range stats.Topics[outboxTopic].Partitions {
		if partitionKey != "-1" {
			m.fetchqCnt.Record(ctx, stats.Topics[outboxTopic].Partitions[partitionKey].FetchqCnt, stats.LabelSet(partitionKey))
			m.fetchqSize.Record(ctx, stats.Topics[outboxTopic].Partitions[partitionKey].FetchqSize, stats.LabelSet(partitionKey))

			if stats.Topics[outboxTopic].Partitions[partitionKey].FetchState != "active" {
				m.fetchState.Record(ctx, int64(1),
					stats.LabelSet(""),
					metric.WithAttributes(attribute.String("fetch_state", stats.Topics[outboxTopic].Partitions[partitionKey].FetchState)))
			} else {
				m.fetchState.Record(ctx, int64(0),
					stats.LabelSet(""),
					metric.WithAttributes(attribute.String("fetch_state", stats.Topics[outboxTopic].Partitions[partitionKey].FetchState)))
			}

			m.loOffset.Record(ctx, stats.Topics[outboxTopic].Partitions[partitionKey].LoOffset, stats.LabelSet(partitionKey))
			m.hiOffset.Record(ctx, stats.Topics[outboxTopic].Partitions[partitionKey].HiOffset, stats.LabelSet(partitionKey))

			m.lsOffset.Record(ctx, stats.Topics[outboxTopic].Partitions[partitionKey].LsOffset, stats.LabelSet(partitionKey))
			m.consumerLag.Record(ctx, stats.Topics[outboxTopic].Partitions[partitionKey].ConsumerLag, stats.LabelSet(partitionKey))
			m.consumerLagStored.Record(ctx, stats.Topics[outboxTopic].Partitions[partitionKey].ConsumerLagStored, stats.LabelSet(partitionKey))
		}
	}

	// cgrp
	if stats.CGRP.State != "up" {
		m.state.Record(ctx, int64(1),
			stats.LabelSet(""),
			metric.WithAttributes(attribute.String("state", stats.CGRP.State)))
	} else {
		m.state.Record(ctx, int64(0),
			stats.LabelSet(""),
			metric.WithAttributes(attribute.String("state", stats.CGRP.State)))
	}
	m.stateAge.Record(ctx, stats.CGRP.StateAge, stats.LabelSet(""))
	m.rebalanceAge.Record(ctx, stats.CGRP.RebalanceAge, stats.LabelSet(""), metric.WithAttributes(attribute.String("last_rebalance_reason", stats.CGRP.RebalanceReason)))
	m.rebalanceCnt.Add(ctx, stats.CGRP.RebalanceCnt, stats.LabelSet(""))
	m.assignmentSize.Record(ctx, stats.CGRP.AssignmentSize, stats.LabelSet(""))
}

// Incr increments a non-stats message based counter
func Incr(counter metric.Int64Counter, operation string, errReason error, extraAttrs ...attribute.KeyValue) {
	ctx := context.Background()
	attrs := []attribute.KeyValue{
		attribute.String("operation", operation),
	}
	if errReason != nil {
		attrs = append(attrs, attribute.String("reason", fmt.Sprint(errReason)))
	}
	attrs = append(attrs, extraAttrs...)
	counter.Add(ctx, 1, metric.WithAttributes(attrs...))
}
