package consumer

import (
	"context"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

const (
	outboxTopic = "outbox.event.kessel.tuples"
	prefix      = "inventory_consumer_"
)

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

type StatsData struct {
	Name     string               `json:"name"`
	ClientID string               `json:"client_id"`
	Replyq   int64                `json:"replyq"`
	Topics   map[string]TopicData `json:"topics"`
	CGRP     CGRPData             `json:"cgrp"`
}

type TopicData struct {
	Topic      string                   `json:"topic"`
	Partitions map[string]PartitionData `json:"partitions"`
}

type PartitionData struct {
	FetchqCnt         int64  `json:"fetchq_cnt"`
	FetchqSize        int64  `json:"fetchq_size"`
	FetchState        string `json:"fetch_state"`
	LoOffset          int64  `json:"lo_offset"`
	HiOffset          int64  `json:"hi_offset"`
	LsOffset          int64  `json:"ls_offset"`
	ConsumerLag       int64  `json:"consumer_lag"`
	ConsumerLagStored int64  `json:"consumer_lag_stored"`
	Rxmsgs            int64  `json:"rxmsgs"`
	Rxbytes           int64  `json:"rxbytes"`
	MsgsInflight      int64  `json:"msgs_inflight"`
}

type CGRPData struct {
	State           string `json:"state"`
	StateAge        int64  `json:"stageage"`
	RebalanceAge    int64  `json:"rebalance_age"`
	RebalanceCnt    int64  `json:"rebalance_cnt"`
	RebalanceReason string `json:"rebalance_reason"`
	AssignmentSize  int64  `json:"assignment_size"`
}

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
	rxmsgs            metric.Int64Counter
	rxbytes           metric.Int64Counter
	msgsInflight      metric.Int64Gauge

	// CGRP Metrics
	state          metric.Int64Gauge
	stateAge       metric.Int64Gauge
	rebalanceAge   metric.Int64Gauge
	rebalanceCnt   metric.Int64Counter
	assignmentSize metric.Int64Gauge
}

func (m *MetricsCollector) New(meter metric.Meter) error {
	var err error

	// create top-level metrics
	if m.replyq, err = meter.Int64Gauge(prefix + "replyq"); err != nil {
		return err
	}

	// create topic.partitions metrics
	if m.fetchqCnt, err = meter.Int64Gauge(prefix + "fetchq_cnt"); err != nil {
		return err
	}
	if m.fetchqSize, err = meter.Int64Gauge(prefix + "fetchq_size"); err != nil {
		return err
	}
	if m.fetchState, err = meter.Int64Gauge(prefix + "fetchq_state"); err != nil {
		return err
	}
	if m.loOffset, err = meter.Int64Gauge(prefix + "lo_offset"); err != nil {
		return err
	}
	if m.hiOffset, err = meter.Int64Gauge(prefix + "hi_offset"); err != nil {
		return err
	}
	if m.lsOffset, err = meter.Int64Gauge(prefix + "ls_offset"); err != nil {
		return err
	}
	if m.consumerLag, err = meter.Int64Gauge(prefix + "consumer_lag"); err != nil {
		return err
	}
	if m.consumerLagStored, err = meter.Int64Gauge(prefix + "consumer_lag_stored"); err != nil {
		return err
	}
	if m.rxmsgs, err = meter.Int64Counter(prefix + "rxmsgs"); err != nil {
		return err
	}
	if m.rxbytes, err = meter.Int64Counter(prefix + "rxbytes"); err != nil {
		return err
	}
	if m.msgsInflight, err = meter.Int64Gauge(prefix + "msgs_inflight"); err != nil {
		return err
	}

	// create cgrp metrics
	if m.state, err = meter.Int64Gauge(prefix + "state"); err != nil {
		return err
	}
	if m.stateAge, err = meter.Int64Gauge(prefix + "stateage"); err != nil {
		return err
	}
	if m.rebalanceAge, err = meter.Int64Gauge(prefix + "rebalance_age"); err != nil {
		return err
	}
	if m.rebalanceCnt, err = meter.Int64Counter(prefix + "rebalance_cnt"); err != nil {
		return err
	}
	if m.assignmentSize, err = meter.Int64Gauge(prefix + "assignment_size"); err != nil {
		return err
	}
	return nil
}

func (m *MetricsCollector) Collect(stats StatsData) {
	// top-level
	ctx := context.Background()
	m.replyq.Record(ctx, stats.Replyq, stats.LabelSet(""))

	// topics.partitions
	for partitionKey, _ := range stats.Topics[outboxTopic].Partitions {
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
			m.rxmsgs.Add(ctx, stats.Topics[outboxTopic].Partitions[partitionKey].Rxmsgs, stats.LabelSet(partitionKey))
			m.rxbytes.Add(ctx, stats.Topics[outboxTopic].Partitions[partitionKey].Rxbytes, stats.LabelSet(partitionKey))
			m.msgsInflight.Record(ctx, stats.Topics[outboxTopic].Partitions[partitionKey].MsgsInflight, stats.LabelSet(partitionKey))
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
