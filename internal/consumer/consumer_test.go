package consumer

import (
	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/project-kessel/inventory-api/cmd/common"
	"github.com/project-kessel/inventory-api/internal/authz"
	"gorm.io/gorm"
)

// Test if Kafka message is received and its fatal or all brokers are down, run is false
// test is Tuple functions fail, loop continues and commit is not done

type TestCase struct {
	name            string
	options         *Options
	config          *Config
	completedConfig CompletedConfig
	inv             InventoryConsumer
	msgPayload      *MessagePayload
	keyPayload      *KeyPayload
	headers         []kafka.Header
	logger          *log.Helper
}

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
	return errs
}
