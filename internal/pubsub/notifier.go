package pubsub

import (
	"context"
)

type Notifier interface {
	Notify(ctx context.Context, payload string) error
}

type PgxNotifier struct {
	driver Driver
}

var _ Notifier = &PgxNotifier{}

func NewPgxNotifier(driver Driver) *PgxNotifier {
	return &PgxNotifier{driver: driver}
}

func (n *PgxNotifier) Notify(ctx context.Context, payload string) error {
	return n.driver.Notify(ctx, payload)
}

type NotifierMock struct{}

var _ Notifier = &NotifierMock{}

func (n *NotifierMock) Notify(ctx context.Context, payload string) error {
	return nil
}
