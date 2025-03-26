package consumer

import (
	"go.opentelemetry.io/otel/metric"
)

type MetricsCollector struct {
	// Top-Level Metrics
	Replyq metric.Int64Gauge

	// Topic.Partitions Metrics
	FetchqCnt         metric.Int64Gauge
	FetchqSize        metric.Int64Gauge
	FetchState        string
	LoOffset          metric.Int64Gauge
	HiOffset          metric.Int64Gauge
	LsOffset          metric.Int64Gauge
	ConsumerLag       metric.Int64Gauge
	ConsumerLagStored metric.Int64Gauge
	Rxmsgs            metric.Int64Counter
	Rxbytes           metric.Int64Counter
	MsgsInflight      metric.Int64Gauge

	// CGRP Metrics
	State           string
	StateAge        metric.Int64Gauge
	JoinState       string
	RebalanceAge    metric.Int64Gauge
	RebalanceCnt    metric.Int64Counter
	RebalanceReason string
	AssignmentSize  metric.Int64Gauge
}

func (m MetricsCollector) New(meter metric.Meter) error {
	var err error

	// create top-level metrics
	if m.Replyq, err = meter.Int64Gauge("replyq", nil); err != nil {
		return err
	}

	// create topic.partitions metrics
	if m.FetchqCnt, err = meter.Int64Gauge("fetchq_cnt", nil); err != nil {
		return err
	}
	if m.FetchqSize, err = meter.Int64Gauge("fetchq_size", nil); err != nil {
		return err
	}
	if m.LoOffset, err = meter.Int64Gauge("lo_offset", nil); err != nil {
		return err
	}
	if m.HiOffset, err = meter.Int64Gauge("hi_offset", nil); err != nil {
		return err
	}
	if m.LsOffset, err = meter.Int64Gauge("ls_offset", nil); err != nil {
		return err
	}
	if m.ConsumerLag, err = meter.Int64Gauge("consumer_lag", nil); err != nil {
		return err
	}
	if m.ConsumerLagStored, err = meter.Int64Gauge("consumer_lag_stored", nil); err != nil {
		return err
	}
	if m.Rxmsgs, err = meter.Int64Counter("rxmsgs", nil); err != nil {
		return err
	}
	if m.Rxbytes, err = meter.Int64Counter("rxbytes", nil); err != nil {
		return err
	}
	if m.MsgsInflight, err = meter.Int64Gauge("msgs_inflight", nil); err != nil {
		return err
	}

	// create cgrp metrics
	if m.StateAge, err = meter.Int64Gauge("stateage", nil); err != nil {
		return err
	}
	if m.RebalanceAge, err = meter.Int64Gauge("rebalance_age", nil); err != nil {
		return err
	}
	if m.RebalanceCnt, err = meter.Int64Counter("rebalance_cnt", nil); err != nil {
		return err
	}
	if m.AssignmentSize, err = meter.Int64Gauge("assignment_size", nil); err != nil {
		return err
	}
	return nil
}
