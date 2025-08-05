package api

import (
	"context"
)

// Producer defines the interface for sending events to a specific destination.
// The destination is determined by the Producer implementation.
type Producer interface {
	// Produce will send the event.  The destination of the event is captured in the Producer implementation.
	Produce(ctx context.Context, event *Event) error
}
