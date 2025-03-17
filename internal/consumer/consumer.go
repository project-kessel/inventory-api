package consumer

import (
	"context"
	"encoding/json"
	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/project-kessel/inventory-api/internal/authz"
	"github.com/project-kessel/inventory-api/internal/authz/api"
	"github.com/project-kessel/inventory-api/internal/storage"
	kessel "github.com/project-kessel/relations-api/api/kessel/relations/v1beta1"
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
			/* TODO this is certainly not production quality
			this should move to using poll and potentially capture
			the number of messages we've fetched before commiting vs
			commiting on every message

			also need to account for other types of messages like when rebalances occur
			rebalance logic should ensure things are committoed before partitions are lost
			*/
			ev, err := i.Consumer.ReadMessage(100 * time.Millisecond)
			if err != nil {
				// Errors are informational and automatically handled by the consumer
				continue
			}
			resp, err := i.CreateTuple(context.Background(), ev)
			if err != nil {
				i.Logger.Infof("Failed to create tuple: %v", err)
			}

			// Just logging the token till i implement the DB actions here
			i.Logger.Infof("Tuple Token: %v", resp)

			// See note above about commiting on every run
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

// CreateTuple calls the Relations API to create a tuple from the message payload received and returns the consistency token
func (i *InventoryConsumer) CreateTuple(ctx context.Context, msg *kafka.Message) (string, error) {

	var rels []*kessel.Relationship
	var tuple *kessel.Relationship

	err := json.Unmarshal(msg.Value, &tuple)
	if err != nil {
		return "", err
	}
	rels = append(rels, tuple)

	resp, err := i.Authorizer.CreateTuples(ctx, &kessel.CreateTuplesRequest{
		Tuples: rels,
	})
	if err != nil {
		return "", err
	}

	return resp.GetConsistencyToken().Token, nil
}

// Shutdown ensures the consumer is properly shutdown, whether by server or due to rebalance
func (i *InventoryConsumer) Shutdown() error {
	// TODO, shutting down the consumer should attempt to commit the offset if we've processed the message
	// for now it just stops the consumer connection
	err := i.Consumer.Close()
	if err != nil {
		i.Logger.Errorf("Error closing kafka consumer: %v", err)
		return err
	}
	return nil
}
