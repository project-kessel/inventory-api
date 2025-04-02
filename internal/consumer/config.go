package consumer

import (
	"fmt"

	"github.com/project-kessel/inventory-api/internal/consumer/auth"
	"github.com/project-kessel/inventory-api/internal/consumer/retry"

	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
)

const clientID = "inventory-consumer"

type Config struct {
	*Options
	KafkaConfig *kafka.ConfigMap

	RetryConfig *retry.Config
	AuthConfig  *auth.Config
}

type completedConfig struct {
	*Options
	Topic                   string
	KafkaConfig             *kafka.ConfigMap
	RetryConfig             *retry.Config
	AuthConfig              *auth.Config
	ReadAfterWriteEnabled   bool
	ReadAfterWriteAllowlist []string
}

type CompletedConfig struct {
	*completedConfig
}

func NewConfig(o *Options) *Config {
	cfg := &Config{
		Options: o,
	}
	cfg.RetryConfig = retry.NewConfig(o.RetryOptions)
	cfg.AuthConfig = auth.NewConfig(o.AuthOptions)
	return cfg
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

		if c.AuthConfig.Enabled {
			if err := config.SetKey("security.protocol", c.AuthConfig.SecurityProtocol); err != nil {
				errs = append(errs, fmt.Errorf("cannot set security.protocol value: %w", err))
			}
			if err := config.SetKey("sasl.mechanism", c.AuthConfig.SASLMechanism); err != nil {
				errs = append(errs, fmt.Errorf("cannot set sasl.mechanism value: %w", err))
			}
			if err := config.SetKey("sasl.username", c.AuthConfig.SASLUsername); err != nil {
				errs = append(errs, fmt.Errorf("cannot set sasl.username value: %w", err))
			}
			if err := config.SetKey("sasl.password", c.AuthConfig.SASLPassword); err != nil {
				errs = append(errs, fmt.Errorf("cannot set sasl.password value: %w", err))
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
		KafkaConfig:             config,
		Topic:                   c.Topic,
		Options:                 c.Options,
		RetryConfig:             c.RetryConfig,
		AuthConfig:              c.AuthConfig,
		ReadAfterWriteEnabled:   c.ReadAfterWriteEnabled,
		ReadAfterWriteAllowlist: c.ReadAfterWriteAllowlist,
		KafkaConfig:             config,
	}}, nil
}
