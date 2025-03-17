package consumer

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/project-kessel/inventory-api/internal/biz/model"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/project-kessel/inventory-api/internal/authz"
	"github.com/project-kessel/inventory-api/internal/authz/api"
	kessel "github.com/project-kessel/relations-api/api/kessel/relations/v1beta1"
	"gorm.io/gorm"
)

// InventoryConsumer defines a Kafka Consumer with required clients and configs to call Relations API and update the Inventory DB with consistency tokens
type InventoryConsumer struct {
	Consumer            *kafka.Consumer
	Config              CompletedConfig
	DB                  *gorm.DB
	PersistenceDisabled bool
	AuthzConfig         authz.CompletedConfig
	Authorizer          api.Authorizer
	Logger              *log.Helper
}

// KeyPayload stores the event key data captured from the topic as emitted by Debezium
type KeyPayload struct {
	MessageSchema map[string]interface{} `json:"schema"`
	InventoryID   string                 `json:"payload"`
}

// MessagePayload stores the event message captured from the topic as emitted by Debezium
type MessagePayload struct {
	MessageSchema  map[string]interface{} `json:"schema"`
	RelationsTuple interface{}            `json:"payload"`
}

// New instantiates a new InventoryConsumer
func New(config CompletedConfig, db *gorm.DB, persistanceDisabled bool, authz authz.CompletedConfig, authorizer api.Authorizer, logger *log.Helper) (InventoryConsumer, error) {
	logger.Info("Setting up kafka consumer")
	consumer, err := kafka.NewConsumer(config.KafkaConfig)
	if err != nil {
		logger.Errorf("Error creating kafka consumer: %v", err)
		return InventoryConsumer{}, err
	}

	return InventoryConsumer{
		Consumer:            consumer,
		Config:              config,
		DB:                  db,
		PersistenceDisabled: persistanceDisabled,
		AuthzConfig:         authz,
		Authorizer:          authorizer,
		Logger:              logger,
	}, nil
}

// Consume begins the consumption loop for the Consumer
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
			/* TODO
			This is certainly not production quality
			this should move to using poll and potentially capture
			the number of messages we've fetched before commiting vs
			commiting on every msg

			Also need to account for other types of messages like when rebalances occur
			rebalance logic should ensure things are committed before partitions are lost
			*/
			msg, err := i.Consumer.ReadMessage(100 * time.Millisecond)
			if err != nil {
				continue
			}

			resp, err := i.CreateTuple(context.Background(), msg)
			if err != nil {
				i.Logger.Infof("Failed to create tuple: %v", err)
				continue
			}

			if !i.PersistenceDisabled {
				// With this logic, msg.Key is expected to be the InventoryID of the resource in Inventory DB
				// and will correlate to aggregateid in the outbox table
				err = i.UpdateConsistencyToken(msg, resp)
				if err != nil {
					i.Logger.Infof("Failed to update consistency token: %v", err)
					continue
				}
			}

			// See note above about commiting on every run
			_, err = i.Consumer.Commit()
			if err != nil {
				i.Logger.Errorf("Error on commit: %v", err)
				continue
			}
			i.Logger.Infof("Consumed event from topic %s, partition %d at offset %s: key = %-10s value = %s\n",
				*msg.TopicPartition.Topic, msg.TopicPartition.Partition, msg.TopicPartition.Offset, string(msg.Key), string(msg.Value))
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
	var msgPayload *MessagePayload
	var tuple *kessel.Relationship

	// As it stands, msg.Value is expected to be a valid JSON body for a single relation
	err := json.Unmarshal(msg.Value, &msgPayload)
	if err != nil {
		return "", fmt.Errorf("error unmarshalling msgPayload: %v", err)
	}

	err = json.Unmarshal([]byte(fmt.Sprintf("%v", msgPayload.RelationsTuple)), &tuple)
	if err != nil {
		return "", fmt.Errorf("error unmarshalling tuple payload: %v", err)
	}

	resp, err := i.Authorizer.CreateTuples(ctx, &kessel.CreateTuplesRequest{
		Tuples: []*kessel.Relationship{tuple},
	})
	if err != nil {
		return "", fmt.Errorf("error creating tuple: %v", err)
	}
	i.Logger.Infof("Tuple Created: %v", tuple)
	return resp.GetConsistencyToken().Token, nil
}

// UpdateConsistencyToken updates the resource in the inventory DB to add the consistency token
func (i *InventoryConsumer) UpdateConsistencyToken(msg *kafka.Message, token string) error {
	//var resource model.Resource
	var msgPayload *KeyPayload

	err := json.Unmarshal(msg.Key, &msgPayload)
	if err != nil {
		return fmt.Errorf("error unmarshalling msgPayload: %v", err)
	}

	i.Logger.Infof("InventoryID is %s", msgPayload.InventoryID)
	//i.DB.First(&resource, "inventory_id = ?", msgPayload.Payload)

	//resource.ConsistencyToken = token
	i.DB.Model(model.Resource{}).Where("inventory_id = ?", msgPayload.InventoryID).Update("consistency_token", token)
	//i.DB.Save(&resource)
	return nil
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
