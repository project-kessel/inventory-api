package pubsub

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/go-kratos/kratos/v2/log"
)

type ListenManagerImpl interface {
	Subscribe(txId string) Subscription
	WaitAndDistribute(ctx context.Context) error
	Run(ctx context.Context) error
}

type Notification struct {
	Channel string `json:"channel"`
	Payload []byte `json:"payload"`
}

type ListenManager struct {
	mu                        sync.RWMutex
	logger                    *log.Helper
	driver                    Driver
	subscriptions             map[string]*subscription
	waitForNotificationCancel context.CancelFunc
}

var _ ListenManagerImpl = &ListenManager{}

func NewListenManager(logger *log.Helper, driver Driver) *ListenManager {
	return &ListenManager{
		mu:                        sync.RWMutex{},
		logger:                    logger,
		driver:                    driver,
		subscriptions:             make(map[string]*subscription),
		waitForNotificationCancel: context.CancelFunc(func() {}),
	}
}

func (l *ListenManager) Subscribe(txId string) Subscription {
	l.mu.Lock()
	defer l.mu.Unlock()

	sub := &subscription{
		txId:          txId,
		listenChan:    make(chan []byte, 2),
		listenManager: l,
	}
	l.subscriptions[txId] = sub

	return sub
}

func (l *ListenManager) WaitAndDistribute(ctx context.Context) error {
	notification, err := func() (*Notification, error) {
		const listenTimeout = 30 * time.Second

		timeoutCtx, cancel := context.WithTimeout(ctx, listenTimeout)
		defer cancel()

		notification, err := l.driver.WaitForNotification(timeoutCtx)
		if err != nil {
			return nil, fmt.Errorf("error waiting for notification: %w", err)
		}

		return notification, nil
	}()
	if err != nil {
		// If the error was a cancellation or the deadline being exceeded but
		// there's no error in the parent context, return no error.
		if (errors.Is(err, context.Canceled) ||
			errors.Is(err, context.DeadlineExceeded)) && ctx.Err() == nil {
			return nil
		}

		return err
	}

	l.mu.RLock()
	defer l.mu.RUnlock()

	for _, sub := range l.subscriptions {
		select {
		case sub.listenChan <- []byte(notification.Payload):
		default:
			l.logger.Error("dropped notification due to full buffer", "payload", notification.Payload)
		}
	}

	return nil
}

func (l *ListenManager) unsubscribe(ctx context.Context, sub *subscription) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	delete(l.subscriptions, sub.txId)

	l.logger.Debugf("removed subscription by txId: %s", sub.txId)

	return nil
}

func (l *ListenManager) Run(ctx context.Context) error {
	for {
		err := l.WaitAndDistribute(ctx)
		if err != nil || ctx.Err() != nil {
			return err
		}
	}
}
