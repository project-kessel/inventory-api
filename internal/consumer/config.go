package consumer

import (
	"fmt"

	"github.com/project-kessel/inventory-api/internal/consumer/auth"
	"github.com/project-kessel/inventory-api/internal/consumer/retry"
	"strings"

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
			authSettings := map[string]string{
				"security.protocol": c.AuthConfig.SecurityProtocol,
				"sasl.mechanism":    c.AuthConfig.SASLMechanism,
				"sasl.username":     c.AuthConfig.SASLUsername,
				"sasl.password":     c.AuthConfig.SASLPassword,
			}
			for key, value := range authSettings {
				if err := config.SetKey(key, value); err != nil {
					errs = append(errs, fmt.Errorf("cannot set %s value: %w", key, err))
				}
			}
		}
		kafkaSettings := map[string]string{
			"client.id":              clientID,
			"bootstrap.servers":      strings.Join(c.BootstrapServers, ","),
			"group.id":               c.ConsumerGroupID,
			"session.timeout.ms":     c.SessionTimeout,
			"heartbeat.interval.ms":  c.HeartbeatInterval,
			"max.poll.interval.ms":   c.MaxPollInterval,
			"enable.auto.commit":     c.EnableAutoCommit,
			"auto.offset.reset":      c.AutoOffsetReset,
			"statistics.interval.ms": c.StatisticsInterval,
		}
		for key, value := range kafkaSettings {
			if err := config.SetKey(key, value); err != nil {
				errs = append(errs, fmt.Errorf("cannot set %s value: %w", key, err))
			}
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
	}}, nil
}
