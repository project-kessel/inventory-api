package consumer

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/project-kessel/inventory-api/internal/authz"
	"github.com/project-kessel/inventory-api/internal/authz/api"
	"github.com/project-kessel/inventory-api/internal/biz/model"
	kessel "github.com/project-kessel/relations-api/api/kessel/relations/v1beta1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"gorm.io/gorm"
	"os"
	"os/signal"
	"syscall"
)

// InventoryConsumer defines a Kafka Consumer with required clients and configs to call Relations API and update the Inventory DB with consistency tokens
type InventoryConsumer struct {
	Consumer    *kafka.Consumer
	Config      CompletedConfig
	DB          *gorm.DB
	AuthzConfig authz.CompletedConfig
	Authorizer  api.Authorizer
	Errors      chan error
	Logger      *log.Helper
}

// KeyPayload stores the event message key captured from the topic as emitted by Debezium
type KeyPayload struct {
	MessageSchema map[string]interface{} `json:"schema"`
	InventoryID   string                 `json:"payload"`
}

// MessagePayload stores the event message value captured from the topic as emitted by Debezium
type MessagePayload struct {
	MessageSchema    map[string]interface{} `json:"schema"`
	RelationsRequest interface{}            `json:"payload"`
}

// New instantiates a new InventoryConsumer
func New(config CompletedConfig, db *gorm.DB, authz authz.CompletedConfig, authorizer api.Authorizer, logger *log.Helper) (InventoryConsumer, error) {
	logger.Info("Setting up kafka consumer")
	consumer, err := kafka.NewConsumer(config.KafkaConfig)
	if err != nil {
		logger.Errorf("error creating kafka consumer: %v", err)
		return InventoryConsumer{}, err
	}

	var errChan chan error
	return InventoryConsumer{
		Consumer:    consumer,
		Config:      config,
		DB:          db,
		AuthzConfig: authz,
		Authorizer:  authorizer,
		Errors:      errChan,
		Logger:      logger,
	}, nil
}

// Consume begins the consumption loop for the Consumer
func (i *InventoryConsumer) Consume() error {
	// TODO -- potentially leverage rebalanceCallback here to run something when rebalance occurs
	// more specifically, if we start commiting after X number of messages, when consumer loses partition
	// we can ensure to commit any offsets not yet commit. This is more futureproofing than critical for now
	err := i.Consumer.SubscribeTopics([]string{i.Config.Topic}, nil)
	if err != nil {
		i.Logger.Errorf("failed to subscribe to topic: %v", err)
		return err
	}

	// Set up a channel for handling exiting pods or ctrl+c
	sigchan := make(chan os.Signal, 1)
	signal.Notify(sigchan, syscall.SIGINT, syscall.SIGTERM)

	// Process messages
	run := true
	i.Logger.Info("Consumer ready: waiting for messages...")
	for run {
		select {
		case sig := <-sigchan:
			i.Logger.Infof("caught signal %v: terminating\n", sig)
			run = false
		default:
			event := i.Consumer.Poll(100)
			if event == nil {
				continue
			}

			switch e := event.(type) {
			case *kafka.Message:
				// capture the operation from the event headers
				var operation string
				var resp interface{}
				for _, v := range e.Headers {
					if v.Key == "operation" {
						operation = string(v.Value)
					}
				}

				switch operation {
				case "created":
					i.Logger.Infof("Operation is: %v", operation)
					resp, err = i.CreateTuple(context.Background(), e.Value)
					if err != nil {
						i.Logger.Infof("failed to create tuple: %v", err)
						continue
					}
				case "updated":
					i.Logger.Infof("Operation is: %v", operation)
					resp, err = i.UpdateTuple(context.Background(), e.Value)
					if err != nil {
						i.Logger.Infof("failed to update tuple: %v", err)
						continue
					}
				case "deleted":
					i.Logger.Infof("Operation is: %v", operation)
					resp, err = i.DeleteTuple(context.Background(), e.Value)
					if err != nil {
						i.Logger.Infof("failed to delete tuple: %v", err)
						continue
					}
				default:
					i.Logger.Infof("unknown operation: %v -- doing nothing", operation)
				}

				err = i.UpdateConsistencyToken(e.Key, fmt.Sprint(resp))
				if err != nil {
					i.Logger.Infof("failed to update consistency token: %v", err)
					continue
				}

				// TODO: Commiting on every message is not ideal - we will need to revisit this as we consume more messages
				// Potentially commit ever X number of messages or use an arbitrary value like:
				// if topicPartition.Offset%10 == 0 { err := c.Commit() }
				_, err = i.Consumer.Commit()
				if err != nil {
					i.Logger.Errorf("error on commit: %v", err)
					continue
				}
				i.Logger.Infof("consumed event from topic %s, partition %d at offset %s: key = %-10s value = %s\n",
					*e.TopicPartition.Topic, e.TopicPartition.Partition, e.TopicPartition.Offset, string(e.Key), string(e.Value))
			case kafka.Error:
				i.Logger.Errorf("consumer error: %v: %v\n", e.Code(), e)
				if e.Code() == kafka.ErrAllBrokersDown || e.IsFatal() {
					run = false
					i.Errors <- e
				}
			default:
				fmt.Printf("event type ignored %v\n", e)
			}
		}
	}
	err = i.Shutdown()
	if err != nil {
		return fmt.Errorf("error in consumer shutdown: %v", err)
	}
	return nil
}

// CreateTuple calls the Relations API to create a tuple from the message payload received and returns the consistency token
func (i *InventoryConsumer) CreateTuple(ctx context.Context, msg []byte) (string, error) {
	var msgPayload *MessagePayload
	var tuple *kessel.Relationship

	// msg value is expected to be a valid JSON body for a single relation
	err := json.Unmarshal(msg, &msgPayload)
	if err != nil {
		return "", fmt.Errorf("error unmarshalling msgPayload: %v", err)
	}

	err = json.Unmarshal([]byte(fmt.Sprintf("%v", msgPayload.RelationsRequest)), &tuple)
	if err != nil {
		return "", fmt.Errorf("error unmarshalling tuple payload: %v", err)
	}

	resp, err := i.Authorizer.CreateTuples(ctx, &kessel.CreateTuplesRequest{
		Tuples: []*kessel.Relationship{tuple},
	})
	if err != nil {
		// If the tuple exists already, capture the token using Check to ensure idempotent updates to tokens in DB
		if status.Convert(err).Code() == codes.AlreadyExists {
			i.Logger.Info("tuple: already exists; fetching consistency token")
			check, err := i.Authorizer.Check(ctx, &kessel.CheckRequest{
				Resource: tuple.Resource,
				Relation: tuple.Relation,
				Subject:  tuple.Subject,
			})
			if err != nil {
				return "", fmt.Errorf("failed to fetch consistency token: %v", err)
			}
			return check.GetConsistencyToken().Token, nil
		} else {
			return "", fmt.Errorf("error creating tuple: %v", err)
		}
	}
	i.Logger.Infof("created tuple: %v", resp)
	return resp.GetConsistencyToken().Token, nil
}

// UpdateTuple calls the Relations API to create a tuple from the message payload received and returns the consistency token
func (i *InventoryConsumer) UpdateTuple(ctx context.Context, msg []byte) (string, error) {
	var msgPayload *MessagePayload
	var tuple *kessel.Relationship

	// msg value is expected to be a valid JSON body for a single relation
	err := json.Unmarshal(msg, &msgPayload)
	if err != nil {
		return "", fmt.Errorf("error unmarshalling msgPayload: %v", err)
	}

	err = json.Unmarshal([]byte(fmt.Sprintf("%v", msgPayload.RelationsRequest)), &tuple)
	if err != nil {
		return "", fmt.Errorf("error unmarshalling tuple payload: %v", err)
	}

	resp, err := i.Authorizer.CreateTuples(ctx, &kessel.CreateTuplesRequest{
		Tuples: []*kessel.Relationship{tuple},
		Upsert: true,
	})
	// TODO: we should understand what kind of errors to look for here in case we need to commit in loop or not
	if err != nil {
		return "", fmt.Errorf("error updating tuple: %v", err)
	}

	i.Logger.Infof("updated tuple: %v", resp)
	return resp.GetConsistencyToken().Token, nil
}

// DeleteTuple calls the Relations API to create a tuple from the message payload received and returns the consistency token
func (i *InventoryConsumer) DeleteTuple(ctx context.Context, msg []byte) (string, error) {
	var msgPayload *MessagePayload
	var filter *kessel.RelationTupleFilter

	// msg value is expected to be a valid JSON body for a single relation
	err := json.Unmarshal(msg, &msgPayload)
	if err != nil {
		return "", fmt.Errorf("error unmarshalling msgPayload: %v", err)
	}

	err = json.Unmarshal([]byte(fmt.Sprintf("%v", msgPayload.RelationsRequest)), &filter)
	if err != nil {
		return "", fmt.Errorf("error unmarshalling tuple payload: %v", err)
	}

	resp, err := i.Authorizer.DeleteTuples(ctx, &kessel.DeleteTuplesRequest{
		Filter: filter,
	})
	if err != nil {
		return "", fmt.Errorf("error deleting tuple: %v", err)
	}
	return resp.GetConsistencyToken().Token, nil
}

// UpdateConsistencyToken updates the resource in the inventory DB to add the consistency token
func (i *InventoryConsumer) UpdateConsistencyToken(msg []byte, token string) error {
	var msgPayload *KeyPayload

	// msg key is expected to be the inventory_id of a resource
	err := json.Unmarshal(msg, &msgPayload)
	if err != nil {
		return fmt.Errorf("error unmarshalling msgPayload: %v", err)
	}

	// this will update all records for the same inventory_id with current consistency token
	i.DB.Model(model.Resource{}).Where("inventory_id = ?", msgPayload.InventoryID).Update("consistency_token", token)
	return nil
}

// Errs returns any errors put on the error channel to ensure proper shutdown of services
func (i *InventoryConsumer) Errs() <-chan error {
	return i.Errors
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
