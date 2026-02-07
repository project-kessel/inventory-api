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
	"sync"
	"syscall"
	"time"

	"github.com/project-kessel/inventory-api/cmd/common"
	"github.com/project-kessel/inventory-api/internal/biz/model"
	"github.com/project-kessel/inventory-api/internal/consumer/auth"
	"github.com/project-kessel/inventory-api/internal/consumer/retry"
	gormrepo "github.com/project-kessel/inventory-api/internal/infrastructure/resourcerepository/gorm"
	"github.com/project-kessel/inventory-api/internal/infrastructure/resourcerepository"
	"github.com/project-kessel/inventory-api/internal/metricscollector"

	"github.com/project-kessel/inventory-api/internal/authz/allow"
	"github.com/project-kessel/inventory-api/internal/authz/kessel"
	"github.com/project-kessel/inventory-api/internal/pubsub"

	"github.com/spf13/viper"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"

	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/project-kessel/relations-api/api/kessel/relations/v1beta1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"gorm.io/gorm"

	"github.com/project-kessel/inventory-api/internal/authz/api"
	"github.com/project-kessel/inventory-api/internal/biz"
	"github.com/project-kessel/inventory-api/internal/biz/model_legacy"
	"go.opentelemetry.io/otel/metric"
)

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
	Authorizer       api.Authorizer
	Errors           chan error
	MetricsCollector *metricscollector.MetricsCollector
	Logger           *log.Helper
	AuthOptions      *auth.Options
	RetryOptions     *retry.Options
	Notifier         pubsub.Notifier
	SchemaService    *model.SchemaService
	// offsetMutex protects OffsetStorage and coordinates offset commit operations
	// to prevent race conditions between shutdown and rebalance callbacks
	offsetMutex sync.Mutex
	// shutdownInProgress indicates if shutdown is currently in progress
	// to coordinate with rebalance callback
	shutdownInProgress bool

	lockToken          string
	lockId             string
	ResourceRepository resourcerepository.ResourceRepository
}

// New instantiates a new InventoryConsumer
func New(config CompletedConfig, db *gorm.DB, schemaRepository model.SchemaRepository, authorizer api.Authorizer, notifier pubsub.Notifier, logger *log.Helper, consumer Consumer) (InventoryConsumer, error) {
	if consumer == nil {
		logger.Info("Setting up kafka consumer")
		logger.Debugf("completed kafka config: %+v", config.KafkaConfig)
		kafkaConsumer, err := kafka.NewConsumer(config.KafkaConfig)
		if err != nil {
			logger.Errorf("error creating kafka consumer: %v", err)
			return InventoryConsumer{}, err
		}
		consumer = kafkaConsumer
	} else {
		logger.Info("Setting up kafka consumer with provided consumer")
	}

	var mc metricscollector.MetricsCollector
	meter := otel.Meter("github.com/project-kessel/inventory-api/blob/main/internal/server/otel")
	err := mc.New(meter)
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
		CACertLocation:   config.AuthConfig.CACertLocation,
	}

	retryOptions := &retry.Options{
		ConsumerMaxRetries:  config.RetryConfig.ConsumerMaxRetries,
		OperationMaxRetries: config.RetryConfig.OperationMaxRetries,
		BackoffFactor:       config.RetryConfig.BackoffFactor,
		MaxBackoffSeconds:   config.RetryConfig.MaxBackoffSeconds,
	}

	var errChan chan error

	maxSerializationRetries := viper.GetInt("storage.max-serialization-retries")
	resourceRepository := gormrepo.NewResourceRepository(db, gormrepo.NewGormTransactionManager(&mc, maxSerializationRetries))
	schemaService := model.NewSchemaService(schemaRepository, logger)

	return InventoryConsumer{
		Consumer:           consumer,
		OffsetStorage:      make([]kafka.TopicPartition, 0),
		Config:             config,
		DB:                 db,
		ResourceRepository: resourceRepository,
		Authorizer:         authorizer,
		Errors:             errChan,
		MetricsCollector:   &mc,
		Logger:             logger,
		AuthOptions:        authnOptions,
		RetryOptions:       retryOptions,
		Notifier:           notifier,
		SchemaService:      schemaService,
		offsetMutex:        sync.Mutex{},
		shutdownInProgress: false,
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
		metricscollector.Incr(i.MetricsCollector.ConsumerErrors, "SubscribeTopics")
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
					metricscollector.Incr(i.MetricsCollector.MsgProcessFailures, "ParseHeaders")
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

				if operation != string(biz.OperationTypeDeleted) {
					inventoryID, err := ParseMessageKey(e.Key)
					if err != nil {
						metricscollector.Incr(i.MetricsCollector.MsgProcessFailures, "ParseMessageKey")
						i.Logger.Errorf("failed to parse message key for for ID: %v", err)
					}
					err = i.UpdateConsistencyTokenIfPresent(inventoryID, fmt.Sprint(resp))
					if err != nil {
						i.Logger.Errorf("failed to update consistency token: %v", err)
						continue
					}
				}

				// if txid is present, we need to notify the producer that we've processed the message
				if !common.IsNil(i.Notifier) && txid != "" {
					err := i.Notifier.Notify(context.Background(), txid)
					if err != nil {
						metricscollector.Incr(i.MetricsCollector.ConsumerErrors, "Notify")
						i.Logger.Errorf("failed to notify producer: %v", err)
						// Do not continue here, we should still commit the offset
					} else {
						i.Logger.Debugf("notified producer of processed message: %s", txid)
					}
				} else {
					i.Logger.Debugf("skipping notification to producer: txid not present or notifier not initialized")
				}

				// store the current offset to be later batch committed
				i.offsetMutex.Lock()
				i.OffsetStorage = append(i.OffsetStorage, e.TopicPartition)
				shouldCommit := checkIfCommit(e.TopicPartition, i.Config.CommitModulo)
				i.offsetMutex.Unlock()
				if shouldCommit {
					err := i.commitStoredOffsets()
					if err != nil {
						metricscollector.Incr(i.MetricsCollector.ConsumerErrors, "commitStoredOffsets")
						i.Logger.Errorf("failed to commit offsets: %v", err)
						continue
					}
				}
				metricscollector.Incr(i.MetricsCollector.MsgsProcessed, operation)
				i.Logger.Infof("consumed event from topic %s, partition %d at offset %s",
					*e.TopicPartition.Topic, e.TopicPartition.Partition, e.TopicPartition.Offset)
				i.Logger.Debugf("consumed event data: key = %-10s value = %s", string(e.Key), string(e.Value))

			case kafka.Error:
				metricscollector.Incr(i.MetricsCollector.KafkaErrorEvents, "kafka",
					attribute.String("code", e.Code().String()))
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
					metricscollector.Incr(i.MetricsCollector.MsgProcessFailures, "StatsCollection")
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
	case string(biz.OperationTypeCreated):
		if relationsEnabled {
			return i.processRelationsOperation(operation, txid, msg, operationConfig{
				fetchRepresentations: func(i *InventoryConsumer, key model.ReporterResourceKey, version *uint) (*model.Representations, *model.Representations, error) {
					return i.ResourceRepository.FindCurrentAndPreviousVersionedRepresentations(nil, key, version, biz.OperationTypeCreated)
				},
				executeSpiceDB: func(i *InventoryConsumer, tuples model.TuplesToReplicate) (string, error) {
					return i.CreateTuple(context.Background(), tuples.TuplesToCreate())
				},
				metricName: "CreateTuple",
			})
		}

	case string(biz.OperationTypeUpdated):
		if relationsEnabled {
			return i.processRelationsOperation(operation, txid, msg, operationConfig{
				fetchRepresentations: func(i *InventoryConsumer, key model.ReporterResourceKey, version *uint) (*model.Representations, *model.Representations, error) {
					return i.ResourceRepository.FindCurrentAndPreviousVersionedRepresentations(nil, key, version, biz.OperationTypeUpdated)
				},
				executeSpiceDB: func(i *InventoryConsumer, tuples model.TuplesToReplicate) (string, error) {
					return i.UpdateTuple(context.Background(), tuples.TuplesToCreate(), tuples.TuplesToDelete())
				},
				metricName: "UpdateTuple",
			})
		}
	case string(biz.OperationTypeDeleted):
		if relationsEnabled {
			return i.processRelationsOperation(operation, txid, msg, operationConfig{
				fetchRepresentations: func(i *InventoryConsumer, key model.ReporterResourceKey, version *uint) (*model.Representations, *model.Representations, error) {
					previous, err := i.ResourceRepository.FindLatestRepresentations(nil, key)
					return nil, previous, err
				},
				executeSpiceDB: func(i *InventoryConsumer, tuples model.TuplesToReplicate) (string, error) {
					_, err := i.DeleteTuple(context.Background(), *tuples.TuplesToDelete())
					return "", err
				},
				metricName: "DeleteTuple",
			})
		}
	default:
		metricscollector.Incr(i.MetricsCollector.MsgProcessFailures, "unknown-operation-type")
		i.Logger.Errorf("unknown operation type, message cannot be processed and will be dropped: offset=%s operation=%s msg=%s",
			msg.TopicPartition.Offset.String(), operation, msg.Value)
	}
	return "", nil
}

type operationConfig struct {
	fetchRepresentations func(i *InventoryConsumer, key model.ReporterResourceKey, version *uint) (*model.Representations, *model.Representations, error)
	executeSpiceDB       func(i *InventoryConsumer, tuples model.TuplesToReplicate) (string, error)
	metricName           string
}

func (i *InventoryConsumer) processRelationsOperation(
	operation string,
	txid string,
	msg *kafka.Message,
	config operationConfig,
) (string, error) {
	i.Logger.Infof("processing message: operation=%s, txid=%s", operation, txid)
	i.Logger.Debugf("processed message tuple=%s", msg.Value)

	tupleEvent, err := ParseMessage(msg.Value, operation)
	if err != nil {
		metricscollector.Incr(i.MetricsCollector.MsgProcessFailures, "ParseMessage")
		i.Logger.Errorf("failed to parse message for tuple: %v", err)
		return "", err
	}

	key := tupleEvent.ReporterResourceKey()

	var currentVersion *uint
	if tupleEvent.CommonVersion() != nil {
		version := tupleEvent.CommonVersion().Uint()
		currentVersion = &version
	}

	current, previous, err := config.fetchRepresentations(i, key, currentVersion)
	if err != nil {
		metricscollector.Incr(i.MetricsCollector.MsgProcessFailures, "FindRepresentations")
		i.Logger.Errorf("failed to find representations: %v", err)
		return "", err
	}

	tuplesToReplicate, err := i.SchemaService.CalculateTuplesForResource(context.Background(), current, previous, key)
	if err != nil {
		metricscollector.Incr(i.MetricsCollector.MsgProcessFailures, "CalculateTuples")
		i.Logger.Errorf("failed to calculate tuples: %v", err)
		return "", err
	}

	if tuplesToReplicate.IsEmpty() {
		return "", nil
	}

	resp, err := i.Retry(func() (string, error) {
		return config.executeSpiceDB(i, tuplesToReplicate)
	}, i.MetricsCollector.MsgProcessFailures)
	if err != nil {
		metricscollector.Incr(i.MetricsCollector.MsgProcessFailures, config.metricName)
		i.Logger.Errorf("failed to %s: %v", config.metricName, err)
		return "", err
	}

	return resp, nil
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

func ParseMessage(msg []byte, operationType string) (*model.TupleEvent, error) {
	var msgPayload *MessagePayload

	// msg value is expected to be a valid JSON body for a single relation
	err := json.Unmarshal(msg, &msgPayload)
	if err != nil {
		return nil, fmt.Errorf("error unmarshaling msgPayload: %v", err)
	}

	payloadJson, err := json.Marshal(msgPayload.RelationsRequest)
	if err != nil {
		return nil, fmt.Errorf("error marshaling tuple payload: %v", err)
	}

	// Now unmarshal into TupleEvent directly (operation type comes from headers)
	var tuple *model.TupleEvent
	err = json.Unmarshal(payloadJson, &tuple)
	if err != nil {
		return nil, fmt.Errorf("error unmarshaling tuple payload: %v", err)
	}
	return tuple, nil
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
func checkIfCommit(partition kafka.TopicPartition, commitModulo int) bool {
	return int(partition.Offset)%commitModulo == 0
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
	i.offsetMutex.Lock()

	// If there are no offsets to commit, return early
	if len(i.OffsetStorage) == 0 {
		i.offsetMutex.Unlock()
		return nil
	}

	// Create a copy of offsets to commit to avoid holding the lock during the commit operation
	offsetsToCommit := make([]kafka.TopicPartition, len(i.OffsetStorage))
	copy(offsetsToCommit, i.OffsetStorage)

	// Clear the storage before releasing the lock to prevent double commits
	i.OffsetStorage = nil

	// Release the lock before the potentially blocking commit operation
	i.offsetMutex.Unlock()

	committed, err := i.Consumer.CommitOffsets(offsetsToCommit)
	if err != nil {
		// Re-acquire lock to restore offsets on failure
		i.offsetMutex.Lock()
		// Only restore if storage is still empty (no new offsets added during commit)
		if len(i.OffsetStorage) == 0 {
			i.OffsetStorage = offsetsToCommit
		} else {
			// Merge the failed offsets back with any new ones
			i.OffsetStorage = append(offsetsToCommit, i.OffsetStorage...)
		}
		i.offsetMutex.Unlock()
		return err
	}
	i.Logger.Infof("offsets committed ([partition:offset]): %s", formatOffsets(committed))
	return nil
}

// CreateTuple calls the Relations API to create a tuple from the message payload received and returns the consistency token
func (i *InventoryConsumer) CreateTuple(ctx context.Context, tuples *[]model.RelationsTuple) (string, error) {
	if tuples == nil || len(*tuples) == 0 {
		return "", fmt.Errorf("no tuples provided")
	}

	relationships, err := i.convertTuplesToRelationships(*tuples)
	if err != nil {
		return "", fmt.Errorf("failed to convert tuples to relationships: %w", err)
	}

	resp, err := i.Authorizer.CreateTuples(ctx, &v1beta1.CreateTuplesRequest{
		Upsert: true,
		Tuples: relationships,
		FencingCheck: &v1beta1.FencingCheck{
			LockId:    i.lockId,
			LockToken: i.lockToken,
		},
	})
	if err != nil {
		if status.Convert(err).Code() == codes.FailedPrecondition {
			i.Logger.Errorf("invalid fencing token: %v", i.lockToken)
			return "", fmt.Errorf("invalid fencing token: %w", err)
		}

		// If the tuple exists already, capture the token using Check to ensure idempotent updates to tokens in DB
		if status.Convert(err).Code() == codes.AlreadyExists {
			i.Logger.Info("tuple already exists; fetching consistency token")

			// Use the first relationship for token fetching
			firstRelationship := relationships[0]
			namespace := firstRelationship.GetResource().GetType().GetNamespace()
			relation := firstRelationship.GetRelation()
			subject := firstRelationship.GetSubject()
			resource := &model_legacy.Resource{
				ResourceType:       firstRelationship.GetResource().GetType().GetName(),
				ReporterResourceId: firstRelationship.GetResource().GetId(),
			}
			_, token, err := i.Authorizer.Check(ctx, namespace, relation, "", resource.ResourceType, resource.ReporterResourceId, subject)
			if err != nil {
				return "", fmt.Errorf("failed to fetch consistency token: %w", err)
			}
			return token.GetToken(), nil
		}
		return "", fmt.Errorf("error creating tuple: %w", err)
	}
	return resp.GetConsistencyToken().GetToken(), nil
}

// UpdateTuple calls the Relations API to create and delete tuples from the message payload received and returns the consistency token
func (i *InventoryConsumer) UpdateTuple(ctx context.Context, tuplesToCreate *[]model.RelationsTuple, tuplesToDelete *[]model.RelationsTuple) (string, error) {
	var token string

	// Create new tuples if any
	if tuplesToCreate != nil && len(*tuplesToCreate) > 0 {
		createToken, err := i.CreateTuple(ctx, tuplesToCreate)
		if err != nil {
			return "", fmt.Errorf("failed to create tuples: %w", err)
		}
		token = createToken
	}

	// Delete old tuples if any
	if tuplesToDelete != nil && len(*tuplesToDelete) > 0 {
		deleteToken, err := i.DeleteTuple(ctx, *tuplesToDelete)
		if err != nil {
			return "", fmt.Errorf("failed to delete tuples: %w", err)
		}
		if token == "" {
			token = deleteToken
		}
	}

	return token, nil
}

// DeleteTuple calls the Relations API to delete tuples from the RelationsTuple slice and returns the consistency token
func (i *InventoryConsumer) DeleteTuple(ctx context.Context, tuples []model.RelationsTuple) (string, error) {
	var token string

	// Delete each tuple
	for _, tuple := range tuples {
		// Convert RelationsTuple to RelationTupleFilter
		filter, err := i.convertTupleToFilter(tuple)
		if err != nil {
			return "", fmt.Errorf("failed to convert tuple to filter: %w", err)
		}

		resp, err := i.Authorizer.DeleteTuples(ctx, &v1beta1.DeleteTuplesRequest{
			Filter: filter,
			FencingCheck: &v1beta1.FencingCheck{
				LockId:    i.lockId,
				LockToken: i.lockToken,
			},
		})
		if err != nil {
			if status.Convert(err).Code() == codes.FailedPrecondition {
				i.Logger.Errorf("invalid fencing token: %v", i.lockToken)
				return "", fmt.Errorf("invalid fencing token: %w", err)
			}
			return "", fmt.Errorf("error deleting tuple: %w", err)
		}

		// Use the latest token
		if token == "" {
			token = resp.GetConsistencyToken().Token
		}
	}

	return token, nil
}

// updateConsistencyTokenIfPresent updates the consistency token in the DB only if the token is non-empty.
// This prevents clearing existing tokens when operations don't generate new tokens (e.g., when there are no tuples to create or delete).
func (i *InventoryConsumer) UpdateConsistencyTokenIfPresent(resourceId, token string) error {
	if token == "" {
		i.Logger.Debugf("skipping consistency token update for resource %s: no token returned", resourceId)
		return nil
	}

	return i.UpdateConsistencyToken(resourceId, token)
}

// UpdateConsistencyToken updates the resource in the inventory DB to add the consistency token
func (i *InventoryConsumer) UpdateConsistencyToken(resourceId, token string) error {
	// this will update all records for the same inventory_id with current consistency token
	result := i.DB.Model(gormrepo.Resource{}).Where("id = ?", resourceId).Update("ktn", token)
	if result.Error != nil {
		metricscollector.Incr(i.MetricsCollector.ConsumerErrors, "UpdateConsistencyToken")
		return result.Error
	}
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

		// Set shutdown flag to coordinate with rebalance callback
		i.offsetMutex.Lock()
		i.shutdownInProgress = true
		hasOffsets := len(i.OffsetStorage) > 0
		i.offsetMutex.Unlock()

		// Commit any remaining offsets before closing
		if hasOffsets {
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
func (i *InventoryConsumer) Retry(operation func() (string, error), metricCounter metric.Int64Counter) (string, error) {
	attempts := 0
	var resp interface{}
	var err error

	for i.RetryOptions.OperationMaxRetries == -1 || attempts < i.RetryOptions.OperationMaxRetries {
		resp, err = operation()
		if err != nil {
			metricscollector.Incr(metricCounter, "Retry")
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

		if len(ev.Partitions) > 0 {
			p := ev.Partitions[0] // there should only be one partition
			i.lockId = fmt.Sprintf("%s/%d", i.Config.ConsumerGroupID, p.Partition)
			i.Logger.Infof("Attempting to acquire lock for lockId: %s", i.lockId)

			lockToken, err := i.Retry(func() (string, error) {
				resp, err := i.Authorizer.AcquireLock(context.Background(), &v1beta1.AcquireLockRequest{
					LockId: i.lockId,
				})
				if err != nil {
					return "", err
				}
				return resp.GetLockToken(), nil
			}, i.MetricsCollector.ConsumerErrors)
			if err != nil {
				i.Logger.Errorf("failed to acquire lock token for %s: %v", i.lockId, err)
				i.lockToken = ""
				return err
			}
			i.lockToken = lockToken
			i.Logger.Infof("Successfully acquired lock token. Token: %s", i.lockToken)
		}

	case kafka.RevokedPartitions:
		i.Logger.Warnf("consumer rebalance event: %d partition(s) revoked: %v\n",
			len(ev.Partitions), ev.Partitions)

		// Check if shutdown is already in progress to avoid double commits
		i.offsetMutex.Lock()
		shutdownInProgress := i.shutdownInProgress
		hasOffsets := len(i.OffsetStorage) > 0
		i.offsetMutex.Unlock()

		if shutdownInProgress {
			i.Logger.Info("shutdown in progress, skipping rebalance offset commit")
			i.lockToken = ""
			i.lockId = ""
			return nil
		}

		if !hasOffsets {
			i.Logger.Debug("no offsets to commit during rebalance")
			i.lockToken = ""
			i.lockId = ""
			return nil
		}

		if i.Consumer.AssignmentLost() {
			i.Logger.Warn("Assignment lost involuntarily, commit may fail")
		}
		err := i.commitStoredOffsets()
		// clear the lock token regardless of commit success/failure
		// since we're losing the partition assignment
		i.lockToken = ""
		i.lockId = ""
		if err != nil {
			i.Logger.Errorf("failed to commit offsets during rebalance: %v", err)
			return err
		}

	default:
		i.Logger.Error("Unexpected event type: %v", event)
	}
	return nil
}

// convertTuplesToRelationships converts a slice of RelationsTuple to v1beta1.Relationship protobuf messages
func (i *InventoryConsumer) convertTuplesToRelationships(tuples []model.RelationsTuple) ([]*v1beta1.Relationship, error) {
	var relationships []*v1beta1.Relationship

	for _, tuple := range tuples {
		relationship, err := i.convertTupleToRelationship(tuple)
		if err != nil {
			return nil, fmt.Errorf("failed to convert tuple %+v: %w", tuple, err)
		}
		relationships = append(relationships, relationship)
	}

	return relationships, nil
}

// convertTupleToRelationship converts a single RelationsTuple to v1beta1.Relationship
func (i *InventoryConsumer) convertTupleToRelationship(tuple model.RelationsTuple) (*v1beta1.Relationship, error) {

	return &v1beta1.Relationship{
		Resource: &v1beta1.ObjectReference{
			Type: &v1beta1.ObjectType{
				Name:      tuple.Resource().Type().Name(),
				Namespace: tuple.Resource().Type().Namespace(), // Use resource type as namespace
			},
			Id: tuple.Resource().Id().Serialize(),
		},
		Relation: tuple.Relation(),
		Subject: &v1beta1.SubjectReference{
			Subject: &v1beta1.ObjectReference{
				Type: &v1beta1.ObjectType{
					Name:      tuple.Subject().Subject().Type().Name(),
					Namespace: tuple.Subject().Subject().Type().Namespace(),
				},
				Id: tuple.Subject().Subject().Id().Serialize(),
			},
		},
	}, nil
}

// convertTupleToFilter converts a model.RelationsTuple to v1beta1.RelationTupleFilter
func (i *InventoryConsumer) convertTupleToFilter(tuple model.RelationsTuple) (*v1beta1.RelationTupleFilter, error) {
	// Store values in variables to take their addresses
	resourceNamespace := tuple.Resource().Type().Namespace()
	resourceType := tuple.Resource().Type().Name()
	resourceId := tuple.Resource().Id().Serialize()
	relation := tuple.Relation()
	subjectNamespace := tuple.Subject().Subject().Type().Namespace()
	subjectType := tuple.Subject().Subject().Type().Name()
	subjectId := tuple.Subject().Subject().Id().Serialize()

	return &v1beta1.RelationTupleFilter{
		ResourceNamespace: &resourceNamespace,
		ResourceType:      &resourceType,
		ResourceId:        &resourceId,
		Relation:          &relation,
		SubjectFilter: &v1beta1.SubjectFilter{
			SubjectNamespace: &subjectNamespace,
			SubjectType:      &subjectType,
			SubjectId:        &subjectId,
		},
	}, nil
}

//Unused at the moment but can be used to convert a v1beta1.RelationTupleFilter to model.RelationsTuple

// // convertFilterToTuple converts a v1beta1.RelationTupleFilter to model.RelationsTuple
// func (i *InventoryConsumer) convertFilterToTuple(filter *v1beta1.RelationTupleFilter) (model.RelationsTuple, error) {
// 	// Extract resource information
// 	resourceId, err := model.NewLocalResourceId(*filter.ResourceId)
// 	if err != nil {
// 		return model.RelationsTuple{}, fmt.Errorf("failed to create resource ID: %w", err)
// 	}
// 	resourceType := model.NewRelationsObjectType(*filter.ResourceType, *filter.ResourceNamespace)
// 	resource := model.NewRelationsResource(resourceId, resourceType)

// 	// Extract subject information
// 	subjectId, err := model.NewLocalResourceId(*filter.SubjectFilter.SubjectId)
// 	if err != nil {
// 		return model.RelationsTuple{}, fmt.Errorf("failed to create subject ID: %w", err)
// 	}
// 	subjectType := model.NewRelationsObjectType(*filter.SubjectFilter.SubjectType, *filter.SubjectFilter.SubjectNamespace)
// 	subjectResource := model.NewRelationsResource(subjectId, subjectType)
// 	subject := model.NewRelationsSubject(subjectResource)

// 	// Create the tuple
// 	return model.NewRelationsTuple(resource, *filter.Relation, subject), nil
// }
