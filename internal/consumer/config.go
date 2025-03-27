package consumer

import (
	"fmt"
	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
)

const clientID = "inventory-consumer"

type Config struct {
	*Options
	KafkaConfig *kafka.ConfigMap
}

type completedConfig struct {
	Topic       string
	KafkaConfig *kafka.ConfigMap
}

type CompletedConfig struct {
	*completedConfig
}

func NewConfig(o *Options) *Config {
	return &Config{
		Options: o,
	}
}

func (c *Config) Complete() (CompletedConfig, []error) {
	var config *kafka.ConfigMap
	var errs []error

	if c.KafkaConfig != nil {
		config = c.KafkaConfig
	} else {
		config = &kafka.ConfigMap{}
		if c.Debug != "" {
			if err := config.SetKey("debug", c.Debug); err != nil {
				errs = append(errs, fmt.Errorf("cannot set debug value: %w", err))
			}
		}
		if err := config.SetKey("client.id", clientID); err != nil {
			errs = append(errs, fmt.Errorf("cannot set client.id value: %w", err))
		}
		if err := config.SetKey("bootstrap.servers", c.BootstrapServers); err != nil {
			errs = append(errs, fmt.Errorf("cannot set bootstrap.servers value: %w", err))
		}
		if err := config.SetKey("group.id", c.ConsumerGroupID); err != nil {
			errs = append(errs, fmt.Errorf("cannot set group.id value: %w", err))
		}
		if err := config.SetKey("session.timeout.ms", c.SessionTimeout); err != nil {
			errs = append(errs, fmt.Errorf("cannot set session.timeout.ms value: %w", err))
		}
		if err := config.SetKey("heartbeat.interval.ms", c.HeartbeatInterval); err != nil {
			errs = append(errs, fmt.Errorf("cannot set heartbeat.interval.ms value: %w", err))
		}
		if err := config.SetKey("max.poll.interval.ms", c.MaxPollInterval); err != nil {
			errs = append(errs, fmt.Errorf("cannot set max.poll.interval.ms value: %w", err))
		}
		if err := config.SetKey("enable.auto.commit", c.EnableAutoCommit); err != nil {
			errs = append(errs, fmt.Errorf("cannot set enable.auto.commit value: %w", err))
		}
		if err := config.SetKey("auto.offset.reset", c.AutoOffsetReset); err != nil {
			errs = append(errs, fmt.Errorf("cannot set auto.offset.reset value: %w", err))
		}
		if err := config.SetKey("statistics.interval.ms", c.StatisticsInterval); err != nil {
			errs = append(errs, fmt.Errorf("cannot set statistics.interval.ms value: %w", err))
		}
	}

	if len(errs) > 0 {
		return CompletedConfig{}, errs
	}

	return CompletedConfig{&completedConfig{
		KafkaConfig: config,
		Topic:       c.Topic,
	}}, nil
}
