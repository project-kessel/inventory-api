package data

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"

	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/uuid"

	"github.com/project-kessel/inventory-api/internal/authz/api"
	"github.com/project-kessel/inventory-api/internal/biz/model"
	kessel "github.com/project-kessel/relations-api/api/kessel/relations/v1beta1"
)

// KafkaEventSource implements EventSource by consuming from Kafka.
// It wraps a Kafka consumer and converts messages to Deliveries with
// acknowledgement semantics for at-least-once delivery.
type KafkaEventSource struct {
	kafkaConfig     *kafka.ConfigMap
	consumer        KafkaConsumer
	topic           string
	consumerGroupID string
	logger          *log.Helper
	authorizer      api.Authorizer

	// Lock state - protected by mutex
	mu        sync.RWMutex
	lockID    model.LockId
	lockToken model.LockToken

	// Offset tracking for acknowledgement
	offsetMu        sync.Mutex
	pendingOffsets  []kafka.TopicPartition
	commitThreshold int
}

// KafkaConsumer is the interface for Kafka consumer operations.
// This matches the confluent-kafka-go Consumer interface methods we need.
type KafkaConsumer interface {
	SubscribeTopics(topics []string, rebalanceCb kafka.RebalanceCb) error
	Poll(timeoutMs int) kafka.Event
	Close() error
	CommitOffsets(offsets []kafka.TopicPartition) ([]kafka.TopicPartition, error)
	AssignmentLost() bool
}

// KafkaEventSourceConfig holds configuration for creating a KafkaEventSource.
type KafkaEventSourceConfig struct {
	KafkaConfig     *kafka.ConfigMap
	Topic           string
	ConsumerGroupID string
	Logger          log.Logger
	Authorizer      api.Authorizer
	CommitThreshold int // Number of acknowledged messages before committing (default 10)
}

// NewKafkaEventSource creates a new KafkaEventSource.
// The Kafka consumer is created lazily when Run is called.
func NewKafkaEventSource(cfg KafkaEventSourceConfig) *KafkaEventSource {
	commitThreshold := cfg.CommitThreshold
	if commitThreshold == 0 {
		commitThreshold = 10
	}

	return &KafkaEventSource{
		kafkaConfig:     cfg.KafkaConfig,
		topic:           cfg.Topic,
		consumerGroupID: cfg.ConsumerGroupID,
		logger:          log.NewHelper(cfg.Logger),
		authorizer:      cfg.Authorizer,
		commitThreshold: commitThreshold,
		pendingOffsets:  make([]kafka.TopicPartition, 0),
	}
}

// Ensure KafkaEventSource implements model.EventSource
var _ model.EventSource = (*KafkaEventSource)(nil)

// Run implements EventSource. It creates a Kafka consumer (if needed),
// subscribes to the topic, and polls for messages, processing each synchronously
// with the provided handler.
//
// Processing guarantees:
//   - Messages are processed in order, one at a time
//   - If handler returns nil, the offset is stored for commit
//   - If handler returns ErrFencingFailed, the offset is NOT stored (will be redelivered)
//   - If handler returns any other error, Run stops and returns the error
//   - Offsets are batch-committed based on commitThreshold
func (k *KafkaEventSource) Run(ctx context.Context, handler model.DeliveryHandler) error {
	// Create consumer if not already set (allows injection for testing)
	if k.consumer == nil {
		kafkaConsumer, err := kafka.NewConsumer(k.kafkaConfig)
		if err != nil {
			return fmt.Errorf("failed to create kafka consumer: %w", err)
		}
		k.consumer = kafkaConsumer
	}

	if err := k.consumer.SubscribeTopics([]string{k.topic}, k.rebalanceCallback); err != nil {
		return fmt.Errorf("failed to subscribe to topic %s: %w", k.topic, err)
	}

	k.logger.Infof("KafkaEventSource started, consuming from topic: %s", k.topic)

	for {
		select {
		case <-ctx.Done():
			k.logger.Info("KafkaEventSource stopping due to context cancellation")
			// Commit any pending offsets before closing
			k.commitPendingOffsets()
			return k.consumer.Close()
		default:
			event := k.consumer.Poll(100)
			if event == nil {
				continue
			}

			switch e := event.(type) {
			case *kafka.Message:
				if err := k.processMessage(ctx, e, handler); err != nil {
					// Fatal error - stop processing
					k.commitPendingOffsets()
					_ = k.consumer.Close()
					return err
				}

			case kafka.Error:
				if e.IsFatal() {
					k.logger.Errorf("fatal Kafka error: %v", e)
					return e
				}
				k.logger.Warnf("recoverable Kafka error: %v", e)

			default:
				// Ignore other event types
			}
		}
	}
}

// processMessage handles a single Kafka message synchronously.
// Returns nil if processing succeeded or was safely skipped (fencing failure).
// Returns an error if processing failed fatally.
func (k *KafkaEventSource) processMessage(ctx context.Context, msg *kafka.Message, handler model.DeliveryHandler) error {
	delivery, err := k.createDelivery(msg)
	if err != nil {
		k.logger.Errorf("failed to create delivery: %v", err)
		// Skip malformed messages - don't commit offset, will be redelivered
		return nil
	}

	// Process synchronously
	err = handler(ctx, delivery)
	if err != nil {
		if errors.Is(err, model.ErrFencingFailed) {
			// Fencing failure - safe to ignore, will be redelivered
			k.logger.Warnf("Fencing condition failed for txid=%s, will be redelivered", delivery.Event().TxID())
			// Do NOT store offset - message will be redelivered after rebalance
			return nil
		}
		// Other error - fatal, stop processing
		return fmt.Errorf("handler error: %w", err)
	}

	// Success - store offset for later commit
	k.storeOffset(msg.TopicPartition)

	k.logger.Infof("Processed message: topic=%s partition=%d offset=%s txid=%s",
		*msg.TopicPartition.Topic, msg.TopicPartition.Partition, msg.TopicPartition.Offset, delivery.Event().TxID())

	return nil
}

// storeOffset stores an offset for later batch commit.
func (k *KafkaEventSource) storeOffset(partition kafka.TopicPartition) {
	k.offsetMu.Lock()
	defer k.offsetMu.Unlock()

	k.pendingOffsets = append(k.pendingOffsets, partition)

	// Commit if we've reached the threshold
	if len(k.pendingOffsets) >= k.commitThreshold {
		k.commitPendingOffsetsLocked()
	}
}

// rebalanceCallback handles partition assignment and revocation.
// It acquires locks on assignment and releases them on revocation.
func (k *KafkaEventSource) rebalanceCallback(consumer *kafka.Consumer, event kafka.Event) error {
	switch ev := event.(type) {
	case kafka.AssignedPartitions:
		k.logger.Infof("Partitions assigned: %v", ev.Partitions)

		if len(ev.Partitions) > 0 {
			// Use the first partition for lock ID (typically single partition)
			p := ev.Partitions[0]
			lockID := model.NewLockId(fmt.Sprintf("%s/%d", k.consumerGroupID, p.Partition))

			k.logger.Infof("Acquiring lock for lockId: %s", lockID)

			if k.authorizer != nil {
				resp, err := k.authorizer.AcquireLock(context.Background(), &kessel.AcquireLockRequest{
					LockId: lockID.String(),
				})
				if err != nil {
					k.logger.Errorf("failed to acquire lock for %s: %v", lockID, err)
					// Clear lock state on failure
					k.mu.Lock()
					k.lockID = ""
					k.lockToken = ""
					k.mu.Unlock()
					return err
				}

				lockToken := model.NewLockToken(resp.GetLockToken())

				k.mu.Lock()
				k.lockID = lockID
				k.lockToken = lockToken
				k.mu.Unlock()

				k.logger.Infof("Successfully acquired lock. LockId: %s, Token: %s", lockID, lockToken)
			}
		}

	case kafka.RevokedPartitions:
		k.logger.Infof("Partitions revoked: %v", ev.Partitions)

		// Commit any pending offsets before losing assignment
		if !k.consumer.AssignmentLost() {
			k.commitPendingOffsets()
		}

		// Clear lock state
		k.mu.Lock()
		k.lockID = ""
		k.lockToken = ""
		k.mu.Unlock()

	default:
		k.logger.Warnf("Unexpected rebalance event: %v", event)
	}

	return nil
}

// createDelivery creates a kafkaDelivery from a Kafka message.
func (k *KafkaEventSource) createDelivery(msg *kafka.Message) (model.Delivery, error) {
	outboxEvent, err := k.parseMessage(msg)
	if err != nil {
		return nil, err
	}

	// Capture the current lock for this delivery
	k.mu.RLock()
	lock := model.NewLock(k.lockID, k.lockToken)
	k.mu.RUnlock()

	return &kafkaDelivery{
		event: outboxEvent,
		lock:  lock,
	}, nil
}

// parseMessage converts a Kafka message to an OutboxEvent.
func (k *KafkaEventSource) parseMessage(msg *kafka.Message) (model.OutboxEvent, error) {
	// Parse headers
	headers := make(map[string]string)
	for _, h := range msg.Headers {
		headers[h.Key] = string(h.Value)
	}

	operation := headers["operation"]
	txid := headers["txid"]

	if operation == "" {
		return model.OutboxEvent{}, fmt.Errorf("missing required 'operation' header")
	}

	// Parse message key for inventory ID
	inventoryID, err := parseMessageKey(msg.Key)
	if err != nil {
		return model.OutboxEvent{}, fmt.Errorf("failed to parse message key: %w", err)
	}

	// Parse message value for tuple event
	tupleEvent, err := parseMessagePayload(msg.Value)
	if err != nil {
		return model.OutboxEvent{}, fmt.Errorf("failed to parse message payload: %w", err)
	}

	// Convert to domain types
	opType := model.OperationType(operation)
	txID := model.NewTransactionId(txid)

	resourceID, err := model.NewResourceId(uuid.MustParse(inventoryID))
	if err != nil {
		return model.OutboxEvent{}, fmt.Errorf("failed to create resource ID: %w", err)
	}

	return model.NewOutboxEvent(opType, txID, resourceID, *tupleEvent), nil
}

// commitPendingOffsets commits all pending offsets.
func (k *KafkaEventSource) commitPendingOffsets() {
	k.offsetMu.Lock()
	defer k.offsetMu.Unlock()
	k.commitPendingOffsetsLocked()
}

// commitPendingOffsetsLocked commits pending offsets (must be called with lock held).
func (k *KafkaEventSource) commitPendingOffsetsLocked() {
	if len(k.pendingOffsets) == 0 {
		return
	}

	committed, err := k.consumer.CommitOffsets(k.pendingOffsets)
	if err != nil {
		k.logger.Errorf("failed to commit offsets: %v", err)
		return
	}

	k.logger.Debugf("Committed %d offsets: %v", len(committed), committed)
	k.pendingOffsets = k.pendingOffsets[:0]
}

// kafkaDelivery implements model.Delivery for Kafka messages.
type kafkaDelivery struct {
	event model.OutboxEvent
	lock  model.Lock
}

// Ensure kafkaDelivery implements model.Delivery
var _ model.Delivery = (*kafkaDelivery)(nil)

// Event returns the underlying OutboxEvent.
func (d *kafkaDelivery) Event() model.OutboxEvent {
	return d.event
}

// Lock returns the fencing lock credentials under which this delivery was received.
func (d *kafkaDelivery) Lock() model.Lock {
	return d.lock
}

// messagePayload matches the Debezium CDC message format.
type messagePayload struct {
	Schema  map[string]interface{} `json:"schema"`
	Payload interface{}            `json:"payload"`
}

// keyPayload matches the message key format.
type keyPayload struct {
	InventoryID string `json:"inventory_id"`
}

func parseMessageKey(data []byte) (string, error) {
	var payload keyPayload
	if err := json.Unmarshal(data, &payload); err != nil {
		return "", fmt.Errorf("error unmarshaling key payload: %w", err)
	}
	return payload.InventoryID, nil
}

func parseMessagePayload(data []byte) (*model.TupleEvent, error) {
	var msgPayload messagePayload
	if err := json.Unmarshal(data, &msgPayload); err != nil {
		return nil, fmt.Errorf("error unmarshaling message payload: %w", err)
	}

	payloadJSON, err := json.Marshal(msgPayload.Payload)
	if err != nil {
		return nil, fmt.Errorf("error marshaling tuple payload: %w", err)
	}

	var tupleEvent model.TupleEvent
	if err := json.Unmarshal(payloadJSON, &tupleEvent); err != nil {
		return nil, fmt.Errorf("error unmarshaling tuple event: %w", err)
	}

	return &tupleEvent, nil
}
