package consumer

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"os"
	"os/signal"
	"slices"
	"strings"
	"syscall"
	"time"

	common "github.com/project-kessel/inventory-api/cmd/common"
	"github.com/project-kessel/inventory-api/internal/consumer/auth"
	"github.com/project-kessel/inventory-api/internal/consumer/retry"
	"github.com/project-kessel/inventory-api/internal/metricscollector"

	"github.com/project-kessel/inventory-api/internal/authz/allow"
	"github.com/project-kessel/inventory-api/internal/authz/kessel"
	"github.com/project-kessel/inventory-api/internal/pubsub"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"

	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/project-kessel/relations-api/api/kessel/relations/v1beta1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"gorm.io/gorm"

	"github.com/project-kessel/inventory-api/internal/authz"
	"github.com/project-kessel/inventory-api/internal/authz/api"
	"github.com/project-kessel/inventory-api/internal/biz/model"
)

// commitModulo is used to define the batch size of offsets based on the current offset being processed
const commitModulo = 10

// defines all required headers for message processing
var requiredHeaders = []string{"operation", "txid"}

var ErrClosed = errors.New("consumer closed")
var ErrMaxRetries = errors.New("max retries reached")

type Consumer interface {
	CommitOffsets(offsets []kafka.TopicPartition) ([]kafka.TopicPartition, error)
	SubscribeTopics(topics []string, rebalanceCb kafka.RebalanceCb) (err error)
	Poll(timeoutMs int) (event kafka.Event)
	IsClosed() bool
	Close() error
	AssignmentLost() bool
}

// InventoryConsumer defines a Consumer with required clients and configs to call Relations API and update the Inventory DB with consistency tokens
type InventoryConsumer struct {
	Consumer         Consumer
	OffsetStorage    []kafka.TopicPartition
	Config           CompletedConfig
	DB               *gorm.DB
	AuthzConfig      authz.CompletedConfig
	Authorizer       api.Authorizer
	Errors           chan error
	MetricsCollector *metricscollector.MetricsCollector
	Logger           *log.Helper
	AuthOptions      *auth.Options
	RetryOptions     *retry.Options
	Notifier         pubsub.Notifier
}

// New instantiates a new InventoryConsumer
func New(config CompletedConfig, db *gorm.DB, authz authz.CompletedConfig, authorizer api.Authorizer, notifier pubsub.Notifier, logger *log.Helper) (InventoryConsumer, error) {
	logger.Info("Setting up kafka consumer")
	logger.Debugf("completed kafka config: %+v", config.KafkaConfig)
	consumer, err := kafka.NewConsumer(config.KafkaConfig)
	if err != nil {
		logger.Errorf("error creating kafka consumer: %v", err)
		return InventoryConsumer{}, err
	}

	var mc metricscollector.MetricsCollector
	meter := otel.Meter("github.com/project-kessel/inventory-api/blob/main/internal/server/otel")
	err = mc.New(meter)
	if err != nil {
		logger.Errorf("error creating metrics collector: %v", err)
		return InventoryConsumer{}, err
	}

	authnOptions := &auth.Options{
		Enabled:          config.AuthConfig.Enabled,
		SecurityProtocol: config.AuthConfig.SecurityProtocol,
		SASLMechanism:    config.AuthConfig.SASLMechanism,
		SASLUsername:     config.AuthConfig.SASLUsername,
		SASLPassword:     config.AuthConfig.SASLPassword,
	}

	retryOptions := &retry.Options{
		ConsumerMaxRetries:  config.RetryConfig.ConsumerMaxRetries,
		OperationMaxRetries: config.RetryConfig.OperationMaxRetries,
		BackoffFactor:       config.RetryConfig.BackoffFactor,
		MaxBackoffSeconds:   config.RetryConfig.MaxBackoffSeconds,
	}

	var errChan chan error

	return InventoryConsumer{
		Consumer:         consumer,
		OffsetStorage:    make([]kafka.TopicPartition, 0),
		Config:           config,
		DB:               db,
		AuthzConfig:      authz,
		Authorizer:       authorizer,
		Errors:           errChan,
		MetricsCollector: &mc,
		Logger:           logger,
		AuthOptions:      authnOptions,
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
	err := i.Consumer.SubscribeTopics([]string{i.Config.Topic}, i.RebalanceCallback)
	if err != nil {
		metricscollector.Incr(i.MetricsCollector.ConsumerErrors, "SubscribeTopics", err)
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
		case <-sigchan:
			run = false
		default:
			event := i.Consumer.Poll(100)
			if event == nil {
				continue
			}

			switch e := event.(type) {
			case *kafka.Message:
				headers, err := ParseHeaders(e)
				if err != nil {
					metricscollector.Incr(i.MetricsCollector.MsgProcessFailures, "ParseHeaders", fmt.Errorf("missing headers"))
					i.Logger.Errorf("failed to parse message headers: %v", err)
					run = false
					continue
				}
				operation := headers["operation"]
				txid := headers["txid"]

				var resp interface{}

				resp, err = i.ProcessMessage(headers, relationsEnabled, e)
				if err != nil {
					i.Logger.Errorf(
						"error processing message: topic=%s partition=%d offset=%s",
						*e.TopicPartition.Topic, e.TopicPartition.Partition, e.TopicPartition.Offset)
					run = false
					continue
				}

				if operation != string(model.OperationTypeDeleted) {
					inventoryID, err := ParseMessageKey(e.Key)
					if err != nil {
						metricscollector.Incr(i.MetricsCollector.MsgProcessFailures, "ParseMessageKey", err)
						i.Logger.Errorf("failed to parse message key for for ID: %v", err)
					}
					err = i.UpdateConsistencyToken(inventoryID, fmt.Sprint(resp))
					if err != nil {
						metricscollector.Incr(i.MetricsCollector.ConsumerErrors, "UpdateConsistencyToken", err)
						i.Logger.Errorf("failed to update consistency token: %v", err)
						continue
					}
				}

				// if txid is present, we need to notify the producer that we've processed the message
				if !common.IsNil(i.Notifier) && txid != "" {
					err := i.Notifier.Notify(context.Background(), txid)
					if err != nil {
						metricscollector.Incr(i.MetricsCollector.ConsumerErrors, "Notify", err)
						i.Logger.Errorf("failed to notify producer: %v", err)
						// Do not continue here, we should still commit the offset
					} else {
						i.Logger.Debugf("notified producer of processed message: %s", txid)
					}
				} else {
					i.Logger.Debugf("skipping notification to producer: txid not present or notifier not initialized")
				}

				// store the current offset to be later batch committed
				i.OffsetStorage = append(i.OffsetStorage, e.TopicPartition)
				if checkIfCommit(e.TopicPartition) {
					err := i.commitStoredOffsets()
					if err != nil {
						metricscollector.Incr(i.MetricsCollector.ConsumerErrors, "commitStoredOffsets", err)
						i.Logger.Errorf("failed to commit offsets: %v", err)
						continue
					}
				}
				metricscollector.Incr(i.MetricsCollector.MsgsProcessed, operation, nil)
				i.Logger.Infof("consumed event from topic %s, partition %d at offset %s",
					*e.TopicPartition.Topic, e.TopicPartition.Partition, e.TopicPartition.Offset)
				i.Logger.Debugf("consumed event data: key = %-10s value = %s", string(e.Key), string(e.Value))

			case kafka.Error:
				metricscollector.Incr(i.MetricsCollector.KafkaErrorEvents, "kafka", nil,
					attribute.String("code", e.Code().String()),
					attribute.String("error", e.Error()))
				if e.IsFatal() {
					run = false
					i.Errors <- e
				} else {
					i.Logger.Errorf("recoverable consumer error: %v: %v -- will retry", e.Code(), e)
					continue
				}

			case *kafka.Stats:
				var stats metricscollector.StatsData
				err = json.Unmarshal([]byte(e.String()), &stats)
				if err != nil {
					metricscollector.Incr(i.MetricsCollector.MsgProcessFailures, "StatsCollection", err)
					i.Logger.Errorf("error unmarshalling stats: %v", err)
					continue
				}
				i.MetricsCollector.Collect(stats)
			default:
				i.Logger.Infof("event type ignored %v", e)
			}
		}
	}
	err = i.Shutdown()
	if !errors.Is(err, ErrClosed) {
		return fmt.Errorf("error in consumer shutdown: %v", err)
	}
	return err
}

func (i *InventoryConsumer) ProcessMessage(headers map[string]string, relationsEnabled bool, msg *kafka.Message) (string, error) {
	operation := headers["operation"]
	txid := headers["txid"]

	switch operation {
	case string(model.OperationTypeCreated):
		i.Logger.Infof("processing message: operation=%s, txid=%s", operation, txid)
		i.Logger.Debugf("processed message tuple=%s", msg.Value)
		if relationsEnabled {
			tuple, err := ParseCreateOrUpdateMessage(msg.Value)
			if err != nil {
				metricscollector.Incr(i.MetricsCollector.MsgProcessFailures, "ParseCreateOrUpdateMessage", err)
				i.Logger.Errorf("failed to parse message for tuple: %v", err)
				return "", err
			}
			resp, err := i.Retry(func() (string, error) {
				return i.CreateTuple(context.Background(), tuple)
			})
			if err != nil {
				metricscollector.Incr(i.MetricsCollector.MsgProcessFailures, "CreateTuple", err)
				i.Logger.Errorf("failed to create tuple: %v", err)
				return "", err
			}
			return resp, nil
		}

	case string(model.OperationTypeUpdated):
		i.Logger.Infof("processing message: operation=%s, txid=%s", operation, txid)
		i.Logger.Debugf("processed message tuple=%s", msg.Value)
		if relationsEnabled {
			tuple, err := ParseCreateOrUpdateMessage(msg.Value)
			if err != nil {
				metricscollector.Incr(i.MetricsCollector.MsgProcessFailures, "ParseCreateOrUpdateMessage", err)
				i.Logger.Errorf("failed to parse message for tuple: %v", err)
				return "", err
			}
			resp, err := i.Retry(func() (string, error) {
				return i.UpdateTuple(context.Background(), tuple)
			})
			if err != nil {
				metricscollector.Incr(i.MetricsCollector.MsgProcessFailures, "UpdateTuple", err)
				i.Logger.Errorf("failed to update tuple: %v", err)
				return "", err
			}
			return resp, nil
		}
	case string(model.OperationTypeDeleted):
		i.Logger.Infof("processing message: operation=%s, txid=%s", operation, txid)
		i.Logger.Debugf("processed message tuple=%s", msg.Value)
		if relationsEnabled {
			filter, err := ParseDeleteMessage(msg.Value)
			if err != nil {
				metricscollector.Incr(i.MetricsCollector.MsgProcessFailures, "ParseDeleteMessage", err)
				i.Logger.Errorf("failed to parse message for filter: %v", err)
				return "", err
			}
			_, err = i.Retry(func() (string, error) {
				return i.DeleteTuple(context.Background(), filter)
			})
			if err != nil {
				metricscollector.Incr(i.MetricsCollector.MsgProcessFailures, "DeleteTuple", err)
				i.Logger.Errorf("failed to delete tuple: %v", err)
				return "", err
			}
			return "", nil
		}
	default:
		metricscollector.Incr(i.MetricsCollector.MsgProcessFailures, "unknown-operation-type", nil)
		i.Logger.Errorf("unknown operation type, message cannot be processed and will be dropped: offset=%s operation=%s msg=%s",
			msg.TopicPartition.Offset.String(), operation, msg.Value)
	}
	return "", nil
}

func ParseHeaders(msg *kafka.Message) (map[string]string, error) {
	headers := make(map[string]string)
	for _, v := range msg.Headers {
		// ignores any extra headers
		if slices.Contains(requiredHeaders, v.Key) {
			headers[v.Key] = string(v.Value)
		}
	}

	// ensures all required header keys are present after parsing, but only operation is required to have a value to process messages
	headerKeys := slices.Sorted(maps.Keys(headers))
	required := slices.Sorted(slices.Values(requiredHeaders))

	if !slices.Equal(headerKeys, required) || headers["operation"] == "" {
		return nil, fmt.Errorf("required headers are missing which would result in message processing failures: %+v", headers)
	}
	return headers, nil
}

func ParseCreateOrUpdateMessage(msg []byte) (*v1beta1.Relationship, error) {
	var msgPayload *MessagePayload
	var tuple *v1beta1.Relationship

	// msg value is expected to be a valid JSON body for a single relation
	err := json.Unmarshal(msg, &msgPayload)
	if err != nil {
		return nil, fmt.Errorf("error unmarshaling msgPayload: %v", err)
	}

	payloadJson, err := json.Marshal(msgPayload.RelationsRequest)
	if err != nil {
		return nil, fmt.Errorf("error marshaling tuple payload: %v", err)
	}

	err = json.Unmarshal(payloadJson, &tuple)
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

	payloadJson, err := json.Marshal(msgPayload.RelationsRequest)
	if err != nil {
		return nil, fmt.Errorf("error marshaling tuple payload: %v", err)
	}

	err = json.Unmarshal(payloadJson, &filter)
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

// checkIfCommit returns true whenever the condition to commit a batch of offsets is met
func checkIfCommit(partition kafka.TopicPartition) bool {
	return partition.Offset%commitModulo == 0
}

// formatOffsets converts a slice of partitions with offset data into a more readable shorthand-coded string to capture what partitions and offsets were comitted
func formatOffsets(offsets []kafka.TopicPartition) string {
	var committedOffsets []string
	for _, partition := range offsets {
		committedOffsets = append(committedOffsets, fmt.Sprintf("[%d:%s]", partition.Partition, partition.Offset.String()))
	}
	return strings.Join(committedOffsets, ",")
}

// commitStoredOffsets commits offsets for all processed messages since last offset commit
func (i *InventoryConsumer) commitStoredOffsets() error {
	committed, err := i.Consumer.CommitOffsets(i.OffsetStorage)
	if err != nil {
		return err
	}

	i.Logger.Infof("offsets committed ([partition:offset]): %s", formatOffsets(committed))
	i.OffsetStorage = nil
	return nil
}

// CreateTuple calls the Relations API to create a tuple from the message payload received and returns the consistency token
func (i *InventoryConsumer) CreateTuple(ctx context.Context, tuple *v1beta1.Relationship) (string, error) {

	resp, err := i.Authorizer.CreateTuples(ctx, &v1beta1.CreateTuplesRequest{
		Tuples: []*v1beta1.Relationship{tuple},
	})
	if err != nil {
		// If the tuple exists already, capture the token using Check to ensure idempotent updates to tokens in DB
		if status.Convert(err).Code() == codes.AlreadyExists {
			i.Logger.Info("tuple already exists; fetching consistency token")

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
	if !i.Consumer.IsClosed() {
		i.Logger.Info("shutting down consumer...")
		if len(i.OffsetStorage) > 0 {
			err := i.commitStoredOffsets()
			if err != nil {
				i.Logger.Errorf("failed to commit offsets before shutting down: %v", err)
			}
		}
		err := i.Consumer.Close()
		if err != nil {
			i.Logger.Errorf("Error closing kafka consumer: %v", err)
			return err
		}
		return ErrClosed
	}
	return ErrClosed
}

// Retry executes the given function and will retry on failure with backoff until max retries is reached
func (i *InventoryConsumer) Retry(operation func() (string, error)) (string, error) {
	attempts := 0
	var resp interface{}
	var err error

	for i.RetryOptions.OperationMaxRetries == -1 || attempts < i.RetryOptions.OperationMaxRetries {
		resp, err = operation()
		if err != nil {
			metricscollector.Incr(i.MetricsCollector.MsgProcessFailures, "Retry", err)
			i.Logger.Errorf("request failed: %v", err)
			attempts++
			if i.RetryOptions.OperationMaxRetries == -1 || attempts < i.RetryOptions.OperationMaxRetries {
				backoff := min(time.Duration(i.RetryOptions.BackoffFactor*attempts*300)*time.Millisecond, time.Duration(i.RetryOptions.MaxBackoffSeconds)*time.Second)
				i.Logger.Errorf("retrying in %v", backoff)
				time.Sleep(backoff)
			}
			continue
		}
		return fmt.Sprintf("%s", resp), nil
	}
	i.Logger.Errorf("Error processing request (max attempts reached: %v): %v", attempts, err)
	return "", ErrMaxRetries
}

// RebalanceCallback logs when rebalance events occur and ensures any stored offsets are committed before losing the partition assignment. It is registered to the kafka 'SubscribeTopics' call and is invoked  automatically whenever rebalances occurs.
// Note, the RebalanceCb function must satisfy the function type func(*Consumer, Event). This function does so, but the consumer embedded in the InventoryConsumer is used versus the passed one which is the same consumer in either case.
func (i *InventoryConsumer) RebalanceCallback(consumer *kafka.Consumer, event kafka.Event) error {
	switch ev := event.(type) {
	case kafka.AssignedPartitions:
		i.Logger.Warnf("consumer rebalance event type: %d new partition(s) assigned: %v\n",
			len(ev.Partitions), ev.Partitions)

	case kafka.RevokedPartitions:
		i.Logger.Warnf("consumer rebalance event: %d partition(s) revoked: %v\n",
			len(ev.Partitions), ev.Partitions)

		if i.Consumer.AssignmentLost() {
			i.Logger.Warn("Assignment lost involuntarily, commit may fail")
		}
		err := i.commitStoredOffsets()
		if err != nil {
			i.Logger.Errorf("failed to commit offsets: %v", err)
			return err
		}

	default:
		i.Logger.Error("Unexpected event type: %v", event)
	}
	return nil
}
