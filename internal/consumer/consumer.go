package consumer

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/project-kessel/inventory-api/internal/consumer/retry"

	"github.com/project-kessel/inventory-api/internal/authz/allow"
	"github.com/project-kessel/inventory-api/internal/authz/kessel"
	"github.com/project-kessel/inventory-api/internal/pubsub"

	"go.opentelemetry.io/otel"

	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/project-kessel/inventory-api/internal/authz"
	"github.com/project-kessel/inventory-api/internal/authz/api"
	"github.com/project-kessel/inventory-api/internal/biz/model"
	"github.com/project-kessel/relations-api/api/kessel/relations/v1beta1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"gorm.io/gorm"
)

var ClosedError = errors.New("consumer closed")

type Consumer interface {
	Consume() error
	CreateTuple(ctx context.Context, tuple *v1beta1.Relationship) (string, error)
	UpdateTuple(ctx context.Context, tuple *v1beta1.Relationship) (string, error)
	DeleteTuple(ctx context.Context, filter *v1beta1.RelationTupleFilter) (string, error)
	UpdateConsistencyToken(inventoryID, token string) error
	Errs() <-chan error
	Shutdown() error
	Retry(operation func() (string, error)) (string, error)
}

// InventoryConsumer defines a Kafka Consumer with required clients and configs to call Relations API and update the Inventory DB with consistency tokens
type InventoryConsumer struct {
	Consumer         *kafka.Consumer
	Config           CompletedConfig
	DB               *gorm.DB
	AuthzConfig      authz.CompletedConfig
	Authorizer       api.Authorizer
	Errors           chan error
	MetricsCollector *MetricsCollector
	Logger           *log.Helper
	RetryOptions     *retry.Options
	Notifier         *pubsub.Notifier
}

// New instantiates a new InventoryConsumer
func New(config CompletedConfig, db *gorm.DB, authz authz.CompletedConfig, authorizer api.Authorizer, notifier *pubsub.Notifier, logger *log.Helper) (InventoryConsumer, error) {
	logger.Info("Setting up kafka consumer")
	consumer, err := kafka.NewConsumer(config.KafkaConfig)
	if err != nil {
		logger.Errorf("error creating kafka consumer: %v", err)
		return InventoryConsumer{}, err
	}

	var mc MetricsCollector
	meter := otel.Meter("github.com/project-kessel/inventory-api/blob/main/internal/server/otel")
	err = mc.New(meter)
	if err != nil {
		logger.Errorf("error creating metrics collector: %v", err)
		return InventoryConsumer{}, err
	}

	retryOptions := &retry.Options{
		ConsumerMaxRetries:  config.RetryConfig.ConsumerMaxRetries,
		OperationMaxRetries: config.RetryConfig.OperationMaxRetries,
		BackoffFactor:       config.RetryConfig.BackoffFactor,
	}

	var errChan chan error

	return InventoryConsumer{
		Consumer:         consumer,
		Config:           config,
		DB:               db,
		AuthzConfig:      authz,
		Authorizer:       authorizer,
		Errors:           errChan,
		MetricsCollector: &mc,
		Logger:           logger,
		RetryOptions:     retryOptions,
		Notifier:         notifier,
	}, nil
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

// Consume begins the consumption loop for the Consumer
func (i *InventoryConsumer) Consume() error {
	// TODO -- potentially leverage rebalanceCallback here to run something when rebalance occurs
	// more specifically, if we start commiting after X number of messages, when consumer loses partition
	// we can ensure to commit any offsets not yet commit. This is more futureproofing than critical for now
	err := i.Consumer.SubscribeTopics([]string{i.Config.Topic}, nil)
	if err != nil {
		i.Logger.Errorf("failed to subscribe to topic: %v", err)
		i.Errors <- err
		return err
	}

	// Set up a channel for handling exiting pods or ctrl+c
	sigchan := make(chan os.Signal, 1)
	signal.Notify(sigchan, syscall.SIGINT, syscall.SIGTERM)

	var relationsEnabled bool
	switch i.Authorizer.(type) {
	case *kessel.KesselAuthz:
		relationsEnabled = true
	case *allow.AllowAllAuthz:
		relationsEnabled = false
	}

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
				var txid string
				var resp interface{}
				for _, v := range e.Headers {
					switch v.Key {
					case "operation":
						operation = string(v.Value)
					case "txid":
						txid = string(v.Value)
					}
				}

				switch operation {
				case string(model.OperationTypeCreated):
					i.Logger.Infof("operation=%s tuple=%s txid=%s", operation, e.Value, txid)
					if relationsEnabled {
						tuple, err := ParseCreateOrUpdateMessage(e.Value)
						if err != nil {
							i.Logger.Errorf("failed to parse message for tuple: %v", err)
						}
						resp, err = i.Retry(func() (string, error) {
							return i.CreateTuple(context.Background(), tuple)
						})
						if err != nil {
							i.Logger.Errorf("failed to create tuple: %v", err)
							run = false
							continue
						}
					}
				case string(model.OperationTypeUpdated):
					i.Logger.Infof("operation=%s tuple=%s txid=%s", operation, e.Value, txid)
					if relationsEnabled {
						tuple, err := ParseCreateOrUpdateMessage(e.Value)
						if err != nil {
							i.Logger.Errorf("failed to parse message for tuple: %v", err)
						}
						resp, err = i.Retry(func() (string, error) {
							return i.UpdateTuple(context.Background(), tuple)
						})
						if err != nil {
							i.Logger.Errorf("failed to update tuple: %v", err)
							run = false
							continue
						}
					}
				case string(model.OperationTypeDeleted):
					i.Logger.Infof("operation=%s tuple=%s", operation, e.Value)
					if relationsEnabled {
						filter, err := ParseDeleteMessage(e.Value)
						if err != nil {
							i.Logger.Errorf("failed to parse message for filter: %v", err)
						}
						_, err = i.Retry(func() (string, error) {
							return i.DeleteTuple(context.Background(), filter)
						})
						if err != nil {
							i.Logger.Errorf("failed to delete tuple: %v", err)
							run = false
							continue
						}
					}
				default:
					i.Logger.Infof("unknown operation: %v -- doing nothing", operation)
				}

				if operation != string(model.OperationTypeDeleted) {
					inventoryID, err := ParseMessageKey(e.Key)
					if err != nil {
						i.Logger.Errorf("failed to parse message key for for ID: %v", err)
					}
					err = i.UpdateConsistencyToken(inventoryID, fmt.Sprint(resp))
					if err != nil {
						i.Logger.Errorf("failed to update consistency token: %v", err)
						continue
					}
				}

				// if txid is present, we need to notify the producer that we've processed the message
				if i.Notifier != nil && txid != "" {
					err := i.Notifier.Notify(context.Background(), txid)
					if err != nil {
						i.Logger.Errorf("failed to notify producer: %v", err)
						// Do not continue here, we should still commit the offset
					} else {
						i.Logger.Debugf("notified producer of processed message: %s" + txid)
					}
				} else {
					i.Logger.Debugf("skipping notification to producer: txid not present or notifier not initialized")
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
				if e.IsFatal() {
					run = false
					i.Errors <- e
				} else {
					i.Logger.Errorf("recoverable consumer error: %v: %v -- will retry\n", e.Code(), e)
					continue
				}

			case *kafka.Stats:
				var stats StatsData
				err = json.Unmarshal([]byte(e.String()), &stats)
				if err != nil {
					i.Logger.Errorf("error unmarshalling stats: %v", err)
					continue
				}
				i.MetricsCollector.Collect(stats)
			default:
				fmt.Printf("event type ignored %v\n", e)
			}
		}
	}
	err = i.Shutdown()
	if !errors.Is(err, ClosedError) {
		return fmt.Errorf("error in consumer shutdown: %v", err)
	}
	return err
}

func ParseCreateOrUpdateMessage(msg []byte) (*v1beta1.Relationship, error) {
	var msgPayload *MessagePayload
	var tuple *v1beta1.Relationship

	// msg value is expected to be a valid JSON body for a single relation
	err := json.Unmarshal(msg, &msgPayload)
	if err != nil {
		return nil, fmt.Errorf("error unmarshaling msgPayload: %v", err)
	}

	err = json.Unmarshal([]byte(fmt.Sprintf("%v", msgPayload.RelationsRequest)), &tuple)
	if err != nil {
		return nil, fmt.Errorf("error unmarshaling tuple payload: %v", err)
	}
	return tuple, nil
}

func ParseDeleteMessage(msg []byte) (*v1beta1.RelationTupleFilter, error) {
	var msgPayload *MessagePayload
	var filter *v1beta1.RelationTupleFilter

	// msg value is expected to be a valid JSON body for a single relation
	err := json.Unmarshal(msg, &msgPayload)
	if err != nil {
		return nil, fmt.Errorf("error unmarshaling msgPayload: %v", err)
	}

	err = json.Unmarshal([]byte(fmt.Sprintf("%v", msgPayload.RelationsRequest)), &filter)
	if err != nil {
		return nil, fmt.Errorf("error unmarshaling tuple payload: %v", err)
	}
	return filter, nil
}

func ParseMessageKey(msg []byte) (string, error) {
	var msgPayload *KeyPayload

	// msg key is expected to be the inventory_id of a resource
	err := json.Unmarshal(msg, &msgPayload)
	if err != nil {
		return "", fmt.Errorf("error unmarshaling msgPayload: %v", err)
	}
	return msgPayload.InventoryID, nil
}

// CreateTuple calls the Relations API to create a tuple from the message payload received and returns the consistency token
func (i *InventoryConsumer) CreateTuple(ctx context.Context, tuple *v1beta1.Relationship) (string, error) {

	resp, err := i.Authorizer.CreateTuples(ctx, &v1beta1.CreateTuplesRequest{
		Tuples: []*v1beta1.Relationship{tuple},
	})
	if err != nil {
		// If the tuple exists already, capture the token using Check to ensure idempotent updates to tokens in DB
		if status.Convert(err).Code() == codes.AlreadyExists {
			i.Logger.Info("tuple: already exists; fetching consistency token")

			namespace := tuple.GetResource().GetType().GetNamespace()
			relation := tuple.GetRelation()
			subject := tuple.GetSubject()
			resource := &model.Resource{
				ResourceType:       tuple.GetResource().GetType().GetName(),
				ReporterResourceId: tuple.GetResource().GetId(),
			}
			_, token, err := i.Authorizer.Check(ctx, namespace, relation, resource, subject)
			if err != nil {
				return "", fmt.Errorf("failed to fetch consistency token: %v", err)
			}
			return token.GetToken(), nil
		}
		return "", fmt.Errorf("error creating tuple: %v", err)
	}
	return resp.GetConsistencyToken().GetToken(), nil
}

// UpdateTuple calls the Relations API to create a tuple from the message payload received and returns the consistency token
func (i *InventoryConsumer) UpdateTuple(ctx context.Context, tuple *v1beta1.Relationship) (string, error) {
	resp, err := i.Authorizer.CreateTuples(ctx, &v1beta1.CreateTuplesRequest{
		Tuples: []*v1beta1.Relationship{tuple},
		Upsert: true,
	})
	// TODO: we should understand what kind of errors to look for here in case we need to commit in loop or not
	if err != nil {
		return "", fmt.Errorf("error updating tuple: %v", err)
	}
	return resp.GetConsistencyToken().Token, nil
}

// DeleteTuple calls the Relations API to create a tuple from the message payload received and returns the consistency token
func (i *InventoryConsumer) DeleteTuple(ctx context.Context, filter *v1beta1.RelationTupleFilter) (string, error) {
	resp, err := i.Authorizer.DeleteTuples(ctx, &v1beta1.DeleteTuplesRequest{
		Filter: filter,
	})
	if err != nil {
		return "", fmt.Errorf("error deleting tuple: %v", err)
	}
	return resp.GetConsistencyToken().Token, nil
}

// UpdateConsistencyToken updates the resource in the inventory DB to add the consistency token
func (i *InventoryConsumer) UpdateConsistencyToken(inventoryID, token string) error {
	// this will update all records for the same inventory_id with current consistency token
	i.DB.Model(model.Resource{}).Where("inventory_id = ?", inventoryID).Update("consistency_token", token)
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
	if !i.Consumer.IsClosed() {
		err := i.Consumer.Close()
		if err != nil {
			i.Logger.Errorf("Error closing kafka consumer: %v", err)
			return err
		}
		return ClosedError
	}
	return ClosedError
}

// Retry executes the given function and will retry on failure with backoff until max retries is reached
func (i *InventoryConsumer) Retry(operation func() (string, error)) (string, error) {
	attempts := 0
	var resp interface{}
	var err error

	for attempts < i.RetryOptions.OperationMaxRetries {
		resp, err = operation()
		if err != nil {
			i.Logger.Errorf("request failed: %v", err)
			attempts++
			if attempts < i.RetryOptions.OperationMaxRetries {
				backoff := time.Duration(i.RetryOptions.BackoffFactor*attempts*300) * time.Millisecond
				i.Logger.Errorf("retrying in %v", backoff)
				time.Sleep(backoff)
			}
			continue
		}
		return fmt.Sprintf("%s", resp), nil
	}
	i.Logger.Errorf("Error processing request (max attempts reached: %v): %v", attempts, err)
	return "", err
}
