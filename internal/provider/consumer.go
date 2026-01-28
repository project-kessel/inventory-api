package provider

import (
	"fmt"
	"strings"

	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
	"github.com/go-kratos/kratos/v2/log"

	authzapi "github.com/project-kessel/inventory-api/internal/authz/api"
	"github.com/project-kessel/inventory-api/internal/biz/model"
	"github.com/project-kessel/inventory-api/internal/data"
)

const consumerClientID = "inventory-consumer"

// ConsumerResult contains the results of consumer initialization.
type ConsumerResult struct {
	// KafkaConfig is the built Kafka configuration map.
	KafkaConfig *kafka.ConfigMap
	// Topic is the Kafka topic to consume from.
	Topic string
	// ConsumerGroupID is the Kafka consumer group ID.
	ConsumerGroupID string
	// RetryConfig contains retry configuration.
	RetryConfig *ConsumerRetryConfig
}

// ConsumerRetryConfig holds retry configuration for the consumer.
type ConsumerRetryConfig struct {
	ConsumerMaxRetries  int
	OperationMaxRetries int
	BackoffFactor       int
	MaxBackoffSeconds   int
}

// BuildConsumerConfig builds the Kafka configuration from options.
// This returns the configuration but does not create the actual consumer.
func BuildConsumerConfig(opts *ConsumerOptions) (*ConsumerResult, error) {
	if !opts.Enabled {
		return nil, nil
	}

	if errs := opts.Validate(); len(errs) > 0 {
		return nil, fmt.Errorf("consumer validation failed: %v", errs)
	}

	config, err := buildKafkaConfigMap(opts)
	if err != nil {
		return nil, err
	}

	return &ConsumerResult{
		KafkaConfig:     config,
		Topic:           opts.Topic,
		ConsumerGroupID: opts.ConsumerGroupID,
		RetryConfig: &ConsumerRetryConfig{
			ConsumerMaxRetries:  opts.Retry.ConsumerMaxRetries,
			OperationMaxRetries: opts.Retry.OperationMaxRetries,
			BackoffFactor:       opts.Retry.BackoffFactor,
			MaxBackoffSeconds:   opts.Retry.MaxBackoffSeconds,
		},
	}, nil
}

// buildKafkaConfigMap builds a kafka.ConfigMap from options.
func buildKafkaConfigMap(opts *ConsumerOptions) (*kafka.ConfigMap, error) {
	config := &kafka.ConfigMap{}
	var errs []error

	if opts.Debug != "" {
		if err := config.SetKey("debug", opts.Debug); err != nil {
			errs = append(errs, fmt.Errorf("cannot set debug value: %w", err))
		}
	}

	if opts.Auth.Enabled {
		authSettings := map[string]string{
			"security.protocol": opts.Auth.SecurityProtocol,
			"sasl.mechanism":    opts.Auth.SASLMechanism,
			"sasl.username":     opts.Auth.SASLUsername,
			"sasl.password":     opts.Auth.SASLPassword,
			"ssl.ca.location":   opts.Auth.CACertLocation,
		}
		for key, value := range authSettings {
			if err := config.SetKey(key, value); err != nil {
				errs = append(errs, fmt.Errorf("cannot set %s value: %w", key, err))
			}
		}
	}

	kafkaSettings := map[string]string{
		"client.id":              consumerClientID,
		"bootstrap.servers":      strings.Join(opts.BootstrapServers, ","),
		"group.id":               opts.ConsumerGroupID,
		"session.timeout.ms":     opts.SessionTimeout,
		"heartbeat.interval.ms":  opts.HeartbeatInterval,
		"max.poll.interval.ms":   opts.MaxPollInterval,
		"enable.auto.commit":     opts.EnableAutoCommit,
		"auto.offset.reset":      opts.AutoOffsetReset,
		"statistics.interval.ms": opts.StatisticsInterval,
	}
	for key, value := range kafkaSettings {
		if err := config.SetKey(key, value); err != nil {
			errs = append(errs, fmt.Errorf("cannot set %s value: %w", key, err))
		}
	}

	if len(errs) > 0 {
		return nil, fmt.Errorf("failed to build kafka config: %v", errs)
	}

	return config, nil
}

// NewKafkaEventSource creates a new KafkaEventSource from options.
// Returns nil if consumer is not enabled.
func NewKafkaEventSource(opts *ConsumerOptions, authorizer authzapi.Authorizer, logger log.Logger) (model.EventSource, error) {
	if !opts.Enabled {
		return nil, nil
	}

	config, err := BuildConsumerConfig(opts)
	if err != nil {
		return nil, err
	}

	return data.NewKafkaEventSource(data.KafkaEventSourceConfig{
		KafkaConfig:     config.KafkaConfig,
		Topic:           config.Topic,
		ConsumerGroupID: config.ConsumerGroupID,
		Logger:          logger,
		Authorizer:      authorizer,
	}), nil
}

// ConsistencyConfig holds consistency configuration derived from options.
type ConsistencyConfig struct {
	ReadAfterWriteEnabled   bool
	ReadAfterWriteAllowlist []string
}

// BuildConsistencyConfig builds consistency configuration from options.
func BuildConsistencyConfig(opts *ConsistencyOptions) *ConsistencyConfig {
	return &ConsistencyConfig{
		ReadAfterWriteEnabled:   opts.ReadAfterWriteEnabled,
		ReadAfterWriteAllowlist: opts.ReadAfterWriteAllowlist,
	}
}
