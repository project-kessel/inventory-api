package api

import (
	"context"
)

type Producer interface {
	Produce(ctx context.Context, event *Event) error
}
