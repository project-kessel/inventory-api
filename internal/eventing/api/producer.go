package api

import (
	"context"
)

type Producer interface {
	// Produce will send the event.  The destination of the event is captured in the Producer implementation.
	Produce(ctx context.Context, event *Event) error
}
