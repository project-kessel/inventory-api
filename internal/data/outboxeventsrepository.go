package data

import (
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/project-kessel/inventory-api/internal"
	"gorm.io/gorm"

	"github.com/project-kessel/inventory-api/internal/biz/model_legacy"
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
			return walOutboxMessage{}, fmt.Errorf("failed to generate uuid for WAL message: %w", err)
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
	return msg, nil
}

// PublishOutboxEvent is a function variable that emits outbox events.
// It defaults to using PostgreSQL WAL logical decoding messages but can be replaced in tests
// as needed (ie; for SQLite where pg_logical_emit_message isn't supported)
var PublishOutboxEvent = publishOutboxEventWAL

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
