package pubsub

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/go-kratos/kratos/v2/log"
)

type ListenManager interface {
	Subscribe(txId string) Subscription
	Run(ctx context.Context) error
}

type Subscription interface {
	NotificationC() <-chan []byte
	Unsubscribe(ctx context.Context)
}

type Notification struct {
	Channel string `json:"channel"`
	Payload []byte `json:"payload"`
}

type subscription struct {
	txId          string
	listenChan    chan []byte
	listenManager *listenManager
	unsubOnce     sync.Once
}

func (s *subscription) NotificationC() <-chan []byte { return s.listenChan }

func (s *subscription) Unsubscribe(ctx context.Context) {
	s.unsubOnce.Do(func() {
		// Unlisten uses background context in case of cancellation.
		if err := s.listenManager.unsubscribe(context.Background(), s); err != nil {
			s.listenManager.logger.Error("error unlistening on channel", "err", err, "txId", s.txId)
		}
	})
}

type listenManager struct {
	mu                        sync.RWMutex
	logger                    *log.Helper
	driver                    Driver
	subscriptions             map[string]*subscription
	waitForNotificationCancel context.CancelFunc
}

func NewListenManager(logger *log.Helper, driver Driver) ListenManager {
	return &listenManager{
		mu:                        sync.RWMutex{},
		logger:                    logger,
		driver:                    driver,
		subscriptions:             make(map[string]*subscription),
		waitForNotificationCancel: context.CancelFunc(func() {}),
	}
}

type channelChange struct {
	channel   string
	close     func()
	operation string
}

// Listen returns a Subscription.
// TODO change to Subscribe
func (l *listenManager) Subscribe(txId string) Subscription {
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

func (l *listenManager) waitAndDistribute(ctx context.Context) error {
	// TODO: handle automatic reconnects
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

func (l *listenManager) unsubscribe(ctx context.Context, sub *subscription) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	delete(l.subscriptions, sub.txId)

	l.logger.Debugf("removed subscription by txId: %s", sub.txId)

	return nil
}

func (l *listenManager) Run(ctx context.Context) error {
	for {
		err := l.waitAndDistribute(ctx)
		if err != nil || ctx.Err() != nil {
			return err
		}
	}
}
