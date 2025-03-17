package consumer

import (
	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/project-kessel/inventory-api/internal/authz"
	"github.com/project-kessel/inventory-api/internal/authz/api"
	"github.com/project-kessel/inventory-api/internal/storage"
	"os"
	"os/signal"
	"syscall"
	"time"
)

type InventoryConsumer struct {
	Consumer    *kafka.Consumer
	Config      CompletedConfig
	DBConfig    storage.CompletedConfig
	AuthzConfig authz.CompletedConfig
	Authorizer  api.Authorizer
	Logger      *log.Helper
}

func New(config CompletedConfig, db storage.CompletedConfig, authz authz.CompletedConfig, authorizer api.Authorizer, logger *log.Helper) (InventoryConsumer, error) {
	logger.Info("Setting up kafka consumer")
	consumer, err := kafka.NewConsumer(config.KafkaConfig)
	if err != nil {
		logger.Errorf("Error creating kafka consumer: %v", err)
		return InventoryConsumer{}, err
	}

	return InventoryConsumer{
		Consumer:    consumer,
		Config:      config,
		DBConfig:    db,
		AuthzConfig: authz,
		Authorizer:  authorizer,
		Logger:      logger,
	}, nil
}

// Consume instantiates a Kafka Consumer to monitor a topic
func (i *InventoryConsumer) Consume() error {
	err := i.Consumer.SubscribeTopics([]string{i.Config.Topic}, nil)
	if err != nil {
		i.Logger.Errorf("Failed to subscribe to topic: %v", err)
		return err
	}

	// Set up a channel for handling exiting pods
	sigchan := make(chan os.Signal, 1)
	signal.Notify(sigchan, syscall.SIGINT, syscall.SIGTERM)

	// Process messages
	run := true
	i.Logger.Info("consumer ready: waiting for messages...")
	for run {
		select {
		case sig := <-sigchan:
			i.Logger.Infof("Caught signal %v: terminating\n", sig)
			run = false
		default:
			// Most examples seem to use Poll() and batch commit after a specific
			// number of messages. This example does so after every message
			ev, err := i.Consumer.ReadMessage(100 * time.Millisecond)
			if err != nil {
				// Errors are informational and automatically handled by the consumer
				continue
			}
			//err = i.WriteDB(ev)
			//if err != nil {
			//	i.Logger.Infof("Failed to write to db: %v", err)
			//}
			_, err = i.Consumer.Commit()
			if err != nil {
				i.Logger.Errorf("Error on commit: %v", err)
			}
			i.Logger.Infof("Consumed event from topic %s, partition %d at offset %s: key = %-10s value = %s\n",
				*ev.TopicPartition.Topic, ev.TopicPartition.Partition, ev.TopicPartition.Offset, string(ev.Key), string(ev.Value))
		}
	}
	err = i.Shutdown()
	if err != nil {
		return err
	}
	return nil
}

func (i *InventoryConsumer) Shutdown() error {
	err := i.Consumer.Close()
	if err != nil {
		i.Logger.Errorf("Error closing kafka consumer: %v", err)
		return err
	}
	return nil
}
