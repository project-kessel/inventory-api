package model

import (
	"testing"

	"github.com/google/uuid"
)

func TestFakeDelivery_Event(t *testing.T) {
	t.Parallel()

	t.Run("should return the underlying event", func(t *testing.T) {
		t.Parallel()

		resourceId, _ := NewResourceId(uuid.New())
		event := NewOutboxEvent(
			OperationCreated,
			NewTransactionId("test-txid"),
			resourceId,
			TupleEvent{},
		)

		delivery := NewFakeDelivery(event)

		if delivery.Event().Operation() != event.Operation() {
			t.Errorf("Expected operation %s, got %s", event.Operation(), delivery.Event().Operation())
		}
		if delivery.Event().TxID() != event.TxID() {
			t.Errorf("Expected txid %s, got %s", event.TxID(), delivery.Event().TxID())
		}
	})
}

func TestFakeDelivery_Lock(t *testing.T) {
	t.Parallel()

	t.Run("should return empty lock when created without lock", func(t *testing.T) {
		t.Parallel()

		resourceId, _ := NewResourceId(uuid.New())
		event := NewOutboxEvent(
			OperationCreated,
			NewTransactionId("test-txid"),
			resourceId,
			TupleEvent{},
		)

		delivery := NewFakeDelivery(event)

		if !delivery.Lock().IsZero() {
			t.Error("Expected empty lock for delivery created without lock")
		}
	})

	t.Run("should return lock when created with lock", func(t *testing.T) {
		t.Parallel()

		resourceId, _ := NewResourceId(uuid.New())
		event := NewOutboxEvent(
			OperationCreated,
			NewTransactionId("test-txid"),
			resourceId,
			TupleEvent{},
		)
		lock := NewLock(NewLockId("consumer/0"), NewLockToken("token123"))

		delivery := NewFakeDeliveryWithLock(event, lock)

		if delivery.Lock().ID() != lock.ID() {
			t.Errorf("Expected lock ID %s, got %s", lock.ID(), delivery.Lock().ID())
		}
		if delivery.Lock().Token() != lock.Token() {
			t.Errorf("Expected lock token %s, got %s", lock.Token(), delivery.Lock().Token())
		}
	})
}

func TestFakeDelivery_ImplementsInterface(t *testing.T) {
	t.Parallel()

	t.Run("FakeDelivery should implement Delivery interface", func(t *testing.T) {
		t.Parallel()

		resourceId, _ := NewResourceId(uuid.New())
		event := NewOutboxEvent(
			OperationCreated,
			NewTransactionId("test-txid"),
			resourceId,
			TupleEvent{},
		)

		// This verifies the interface is implemented at compile time
		var delivery Delivery = NewFakeDelivery(event)

		// Verify we can call the interface methods
		_ = delivery.Event()
		_ = delivery.Lock()
	})
}
