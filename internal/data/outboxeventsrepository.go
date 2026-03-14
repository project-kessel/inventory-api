package data

import (
	"encoding/json"
	"fmt"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/uuid"
	"github.com/project-kessel/inventory-api/internal"
	"gorm.io/gorm"

	"github.com/project-kessel/inventory-api/internal/biz/model_legacy"
	"github.com/project-kessel/inventory-api/internal/storage"
)

// walOutboxMessage defines the content value of a logical decoding message
// it mirrors the legacy outbox table to make the transition transparent to consumer processes
type walOutboxMessage struct {
	ID            string              `json:"id"`
	AggregateType string              `json:"aggregatetype"`
	AggregateID   string              `json:"aggregateid"`
	Operation     string              `json:"operation"`
	TxID          string              `json:"txid"`
	Payload       internal.JsonObject `json:"payload"`
}

// mapOutboxEventToWALMessage maps an OutboxEvent to a WAL message ensuring proper conversion of types
func mapOutboxEventToWALMessage(event *model_legacy.OutboxEvent) (walOutboxMessage, error) {
	if event.ID == uuid.Nil {
		id, err := uuid.NewV7()
		if err != nil {
			return walOutboxMessage{}, fmt.Errorf(
				"failed to generate uuid for WAL message: err=%w, event=%v", err, event.Payload)
		}
		event.ID = id
	}

	msg := walOutboxMessage{
		ID:            event.ID.String(),
		AggregateType: string(event.AggregateType),
		AggregateID:   event.AggregateID,
		Operation:     string(event.Operation.OperationType()),
		TxID:          event.TxId,
		Payload:       event.Payload,
	}
	log.Infof("outbox event successfully converted to WAL message: id=%s, aggregateType=%s, aggregateid=%s",
		msg.ID, msg.AggregateType, msg.AggregateID,
	)
	log.Debugf("operation=%s, txid=%s, payload=%v", msg.Operation, msg.TxID, msg.Payload)

	return msg, nil
}

// OutboxPublisher is a function type for emitting outbox events.
// Tests can provide a no-op implementation for SQLite compatibility.
type OutboxPublisher func(tx *gorm.DB, event *model_legacy.OutboxEvent) error

// SetOutboxPublisher returns the appropriate OutboxPublisher for the given mode.
func SetOutboxPublisher(mode string) OutboxPublisher {
	switch mode {
	case storage.OutboxModeWAL:
		log.Info("Using WAL logical decoding message outbox publisher")
		return publishOutboxEventWAL
	default:
		log.Info("Using table-based outbox publisher")
		return publishOutboxEvent
	}
}

// publishOutboxEventWAL publishes an event to WAL using logical decoding messages
func publishOutboxEventWAL(tx *gorm.DB, event *model_legacy.OutboxEvent) error {
	msg, err := mapOutboxEventToWALMessage(event)
	if err != nil {
		return fmt.Errorf("failed to create WAL message: %w", err)
	}

	content, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal WAL message: %w", err)
	}

	// sets the prefix on the logical message to match aggregatetype
	// this ensures outbox router smt still properly routes to the correct topics as before
	prefix := string(event.AggregateType)

	// the first arg to pg_logical_emit_message is set to 'true' to ensure the message is part of
	// the current transaction, meaning it only appears in the WAL if the surrounding transaction commits.
	return tx.Exec(
		"SELECT pg_logical_emit_message(true, ?, ?)", prefix, string(content),
	).Error
}

// original outbox implementation, kept to allow for flipping
// to new WAL version using flag to simplify rollout
func publishOutboxEvent(tx *gorm.DB, event *model_legacy.OutboxEvent) error {
	if err := tx.Create(event).Error; err != nil {
		return err
	}
	if err := tx.Delete(event).Error; err != nil {
		return err
	}
	return nil
}
