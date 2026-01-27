package model

// Delivery provides access to an OutboxEvent and its fencing context.
// Deliveries are processed synchronously by a DeliveryHandler.
// The handler's return value determines acknowledgement:
//   - nil: success, offset will be committed
//   - ErrFencingFailed: safe failure, offset NOT committed (redelivery)
//   - other error: fatal, processing stops
type Delivery interface {
	// Event returns the underlying OutboxEvent.
	Event() OutboxEvent

	// Lock returns the fencing lock credentials under which this delivery was received.
	// The lock should be used when replicating to the relations service to ensure
	// fencing semantics are maintained.
	Lock() Lock
}

// FakeDelivery provides a simple implementation of Delivery for testing.
type FakeDelivery struct {
	event OutboxEvent
	lock  Lock
}

// NewFakeDelivery creates a new FakeDelivery wrapping the given event with an empty lock.
func NewFakeDelivery(event OutboxEvent) *FakeDelivery {
	return &FakeDelivery{
		event: event,
	}
}

// NewFakeDeliveryWithLock creates a new FakeDelivery wrapping the given event and lock.
func NewFakeDeliveryWithLock(event OutboxEvent, lock Lock) *FakeDelivery {
	return &FakeDelivery{
		event: event,
		lock:  lock,
	}
}

// Event returns the underlying OutboxEvent.
func (d *FakeDelivery) Event() OutboxEvent {
	return d.event
}

// Lock returns the fencing lock credentials.
func (d *FakeDelivery) Lock() Lock {
	return d.lock
}

// Ensure FakeDelivery implements Delivery
var _ Delivery = (*FakeDelivery)(nil)
