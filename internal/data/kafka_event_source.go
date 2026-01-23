package data

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/uuid"

	"github.com/project-kessel/inventory-api/internal/biz/model"
)

// KafkaEventSource implements EventSource by consuming from Kafka.
// It wraps a Kafka consumer and converts messages to OutboxEvents.
type KafkaEventSource struct {
	consumer KafkaConsumer
	topic    string
	logger   *log.Helper
}

// KafkaConsumer is the interface for Kafka consumer operations.
// This matches the confluent-kafka-go Consumer interface methods we need.
type KafkaConsumer interface {
	SubscribeTopics(topics []string, rebalanceCb kafka.RebalanceCb) error
	Poll(timeoutMs int) kafka.Event
	Close() error
}

// KafkaEventSourceConfig holds configuration for creating a KafkaEventSource.
type KafkaEventSourceConfig struct {
	Consumer KafkaConsumer
	Topic    string
	Logger   log.Logger
}

// NewKafkaEventSource creates a new KafkaEventSource.
func NewKafkaEventSource(cfg KafkaEventSourceConfig) *KafkaEventSource {
	return &KafkaEventSource{
		consumer: cfg.Consumer,
		topic:    cfg.Topic,
		logger:   log.NewHelper(cfg.Logger),
	}
}

// Ensure KafkaEventSource implements EventSource
var _ EventSource = (*KafkaEventSource)(nil)

// Run implements EventSource. It subscribes to the topic and polls for messages,
// converting each to an OutboxEvent and calling emit.
func (k *KafkaEventSource) Run(ctx context.Context, emit func(model.OutboxEvent)) error {
	if err := k.consumer.SubscribeTopics([]string{k.topic}, nil); err != nil {
		return fmt.Errorf("failed to subscribe to topic %s: %w", k.topic, err)
	}

	k.logger.Infof("KafkaEventSource started, consuming from topic: %s", k.topic)

	for {
		select {
		case <-ctx.Done():
			k.logger.Info("KafkaEventSource stopping due to context cancellation")
			return k.consumer.Close()
		default:
			event := k.consumer.Poll(100)
			if event == nil {
				continue
			}

			switch e := event.(type) {
			case *kafka.Message:
				outboxEvent, err := k.parseMessage(e)
				if err != nil {
					k.logger.Errorf("failed to parse message: %v", err)
					continue
				}
				emit(outboxEvent)

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
