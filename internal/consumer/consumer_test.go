package consumer

import (
	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/project-kessel/inventory-api/cmd/common"
	"github.com/project-kessel/inventory-api/internal/authz"
	"github.com/project-kessel/inventory-api/internal/consumer/retry"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/otel"
	"gorm.io/gorm"
	"testing"
)

// Matches expected defaults set during NewOptions calls
const (
	DefaultClientID            = "inventory-consumer"
	DefaultEnabled             = true
	DefaultBootstrapServers    = "localhost:9092"
	DefaultConsumerGroupID     = "inventory-consumer"
	DefaultTopic               = "outbox.event.kessel.tuples"
	DefaultSessionTimeout      = "45000"
	DefaultHeartbeatInterval   = "3000"
	DefaultMaxPollInterval     = "300000"
	DefaultEnableAutoCommit    = "false"
	DefaultAutoOffsetReset     = "earliest"
	DefaultStatisticsInterval  = "60000"
	DefaultDebug               = ""
	DefaultConsumerMaxRetries  = "3"
	DefaultOperationMaxRetries = "3"
	DefaultBackoffFactor       = "4"
)

type TestCase struct {
	name            string
	description     string
	options         *Options
	config          *Config
	completedConfig CompletedConfig
	inv             InventoryConsumer
	metrics         MetricsCollector
	logger          *log.Helper
}

// TestSetup creates a test struct that calls most of the initial constructor methods we intend to test in unit tests.
func (t *TestCase) TestSetup() []error {
	t.options = NewOptions()
	t.options.BootstrapServers = "localhost:9092"
	t.config = NewConfig(t.options)

	_, logger := common.InitLogger("info", common.LoggerOptions{})
	t.logger = log.NewHelper(log.With(logger, "subsystem", "inventoryConsumer"))

	var errs []error
	var err error

	errs = t.options.Complete()
	errs = t.options.Validate()
	t.completedConfig, errs = NewConfig(t.options).Complete()

	t.inv, err = New(t.completedConfig, &gorm.DB{}, authz.CompletedConfig{}, nil, t.logger)
	if err != nil {
		errs = append(errs, err)
	}
	err = t.metrics.New(otel.Meter("github.com/project-kessel/inventory-api/blob/main/internal/server/otel"))
	if err != nil {
		errs = append(errs, err)
	}
	return errs
}

func TestOptions_NewOptions(t *testing.T) {
	test := TestCase{
		name:        "TestNewOptions_DefaultSettings",
		description: "ensures default options are properly set",
	}
	var errs []error
	errs = test.TestSetup()
	assert.Nil(t, errs)

	t.Run(test.name, func(t *testing.T) {
		expected := &Options{
			Enabled:            DefaultEnabled,
			BootstrapServers:   DefaultBootstrapServers,
			ConsumerGroupID:    DefaultConsumerGroupID,
			Topic:              DefaultTopic,
			SessionTimeout:     DefaultSessionTimeout,
			HeartbeatInterval:  DefaultHeartbeatInterval,
			MaxPollInterval:    DefaultMaxPollInterval,
			EnableAutoCommit:   DefaultEnableAutoCommit,
			AutoOffsetReset:    DefaultAutoOffsetReset,
			StatisticsInterval: DefaultStatisticsInterval,
			Debug:              DefaultDebug,
			RetryOptions: &retry.Options{
				ConsumerMaxRetries:  DefaultConsumerMaxRetries,
				OperationMaxRetries: DefaultOperationMaxRetries,
				BackoffFactor:       DefaultBackoffFactor,
			},
		}
		assert.Equal(t, expected, test.options)
	})
}

func TestConfig_CompletedConfig(t *testing.T) {
	test := TestCase{
		name:        "TestCompletedConfig",
		description: "ensures completedConfig has all fields set and is set to default values",
	}
	var errs []error
	errs = test.TestSetup()
	assert.Nil(t, errs)

	t.Run(test.name, func(t *testing.T) {
		expected := CompletedConfig{
			completedConfig: &completedConfig{
				Topic: "outbox.event.kessel.tuples",
				KafkaConfig: &kafka.ConfigMap{
					"client.id":              DefaultClientID,
					"bootstrap.servers":      DefaultBootstrapServers,
					"group.id":               DefaultConsumerGroupID,
					"session.timeout.ms":     DefaultSessionTimeout,
					"heartbeat.interval.ms":  DefaultHeartbeatInterval,
					"max.poll.interval.ms":   DefaultMaxPollInterval,
					"enable.auto.commit":     DefaultEnableAutoCommit,
					"auto.offset.reset":      DefaultAutoOffsetReset,
					"statistics.interval.ms": DefaultStatisticsInterval,
				},
			},
		}
		assert.Equal(t, expected.completedConfig.Topic, test.completedConfig.Topic)
		assert.Equal(t, *expected.completedConfig.KafkaConfig, *test.completedConfig.KafkaConfig)
	})
}

func TestParseCreateOrUpdateMessage(t *testing.T) {
	testMsg := `{"schema":{"type":"string","optional":false,"name":"io.debezium.data.Json","version":1},"payload":"{\"subject\":{\"subject\":{\"id\":\"1234\", \"type\":{\"name\":\"workspace\",\"namespace\":\"rbac\"}}},\"relation\":\"t_workspace\",\"resource\":{\"id\":\"4321\",\"type\":{\"name\":\"integration\",\"namespace\":\"notifications\"}}}"}`
	tuple, err := ParseCreateOrUpdateMessage([]byte(testMsg))
	assert.Nil(t, err)
	assert.Equal(t, tuple.Subject.Subject.Id, "1234")
	assert.Equal(t, tuple.Subject.Subject.Type.Name, "workspace")
	assert.Equal(t, tuple.Subject.Subject.Type.Namespace, "rbac")
	assert.Equal(t, tuple.Relation, "t_workspace")
	assert.Equal(t, tuple.Resource.Id, "4321")
	assert.Equal(t, tuple.Resource.Type.Name, "integration")
	assert.Equal(t, tuple.Resource.Type.Namespace, "notifications")
}

func TestParseDeleteMessage(t *testing.T) {
	testMsg := `{"schema":{"type":"string","optional":false,"name":"io.debezium.data.Json","version":1},"payload":"{\"resource_id\":\"4321\",\"resource_type\":\"integration\",\"resource_namespace\":\"notifications\",\"relation\":\"t_workspace\",\"subject_filter\":{\"subject_type\":\"workspace\",\"subject_namespace\":\"rbac\",\"subject_id\":\"1234\"}}"}`
	filter, err := ParseDeleteMessage([]byte(testMsg))
	assert.Nil(t, err)
	assert.Equal(t, *filter.ResourceId, "4321")
	assert.Equal(t, *filter.ResourceType, "integration")
	assert.Equal(t, *filter.ResourceNamespace, "notifications")
	assert.Equal(t, *filter.Relation, "t_workspace")
	assert.Equal(t, *filter.SubjectFilter.SubjectId, "1234")
	assert.Equal(t, *filter.SubjectFilter.SubjectType, "workspace")
	assert.Equal(t, *filter.SubjectFilter.SubjectNamespace, "rbac")
}

func TestParseMessageKey(t *testing.T) {
	testMsg := `{"schema":{"type":"string","optional":false},"payload":"00000000-0000-0000-0000-000000000000"}`
	key, err := ParseMessageKey([]byte(testMsg))
	assert.Nil(t, err)
	assert.Equal(t, key, "00000000-0000-0000-0000-000000000000")
}
