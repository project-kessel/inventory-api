package pubsub

import (
	"context"
)

type Notifier struct {
	driver Driver
}

func NewNotifier(driver Driver) *Notifier {
	return &Notifier{driver: driver}
}

func (n *Notifier) Notify(ctx context.Context, payload string) error {
	return n.driver.Notify(ctx, payload)
}
