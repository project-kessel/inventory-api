package data

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/project-kessel/inventory-api/internal/biz/model"
	"github.com/project-kessel/inventory-api/internal/pubsub"
)

// PgxClusterBroadcast implements model.ClusterBroadcast using Postgres LISTEN/NOTIFY
// for cross-process notification and in-memory channels for same-process fast-path.
//
// It uses two separate database connections: one dedicated to LISTEN (which blocks
// while waiting for notifications) and another for NOTIFY operations. This prevents
// NOTIFY calls from being blocked while waiting for incoming notifications.
type PgxClusterBroadcast struct {
	listenerDriver pubsub.Driver
	notifierDriver pubsub.Driver
	logger         *log.Helper

	mu            sync.RWMutex
	subscriptions map[string]chan struct{}
}

// PgxClusterBroadcastConfig holds configuration for creating a PgxClusterBroadcast.
type PgxClusterBroadcastConfig struct {
	// ListenerDriver is used for LISTEN/WaitForNotification (dedicated connection).
	ListenerDriver pubsub.Driver
	// NotifierDriver is used for NOTIFY operations (separate connection).
	NotifierDriver pubsub.Driver
	Logger         log.Logger
}

// NewPgxClusterBroadcast creates a new PgxClusterBroadcast.
func NewPgxClusterBroadcast(cfg PgxClusterBroadcastConfig) *PgxClusterBroadcast {
	var logger *log.Helper
	if cfg.Logger != nil {
		logger = log.NewHelper(cfg.Logger)
	}

	return &PgxClusterBroadcast{
		listenerDriver: cfg.ListenerDriver,
		notifierDriver: cfg.NotifierDriver,
		logger:         logger,
		subscriptions:  make(map[string]chan struct{}),
	}
}

// Ensure PgxClusterBroadcast implements model.ClusterBroadcast
var _ model.ClusterBroadcast = (*PgxClusterBroadcast)(nil)

// Start initializes the broadcast mechanism by connecting both drivers to Postgres
// and starting the notification listener loop. This should be called once at
// application startup.
func (b *PgxClusterBroadcast) Start(ctx context.Context) error {
	// Connect the listener driver and start listening
	if b.listenerDriver != nil {
		if err := b.listenerDriver.Connect(ctx); err != nil {
			return fmt.Errorf("failed to connect listener driver: %w", err)
		}

		if err := b.listenerDriver.Listen(ctx); err != nil {
			return fmt.Errorf("failed to start listening: %w", err)
		}

		go b.runNotificationLoop(ctx)
	}

	// Connect the notifier driver (separate connection for NOTIFY operations)
	if b.notifierDriver != nil {
		if err := b.notifierDriver.Connect(ctx); err != nil {
			return fmt.Errorf("failed to connect notifier driver: %w", err)
		}
	}

	return nil
}

// Wait blocks until a signal for the given key is received, or the context
// is cancelled. This works both in-process (via Signal in the same process)
// and cross-process (via Postgres LISTEN/NOTIFY).
func (b *PgxClusterBroadcast) Wait(ctx context.Context, key model.SignalKey) error {
	ch := make(chan struct{}, 1)
	keyStr := string(key)

	b.mu.Lock()
	b.subscriptions[keyStr] = ch
	b.mu.Unlock()

	defer func() {
		b.mu.Lock()
		delete(b.subscriptions, keyStr)
		b.mu.Unlock()
	}()

	select {
	case <-ch:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("wait cancelled: %w", ctx.Err())
	}
}

// Signal broadcasts a signal for the given key to all waiting listeners.
// Since each key has exactly one waiter, if we find it in-process we can skip
// the cluster broadcast. Otherwise, we send via Postgres NOTIFY.
func (b *PgxClusterBroadcast) Signal(ctx context.Context, key model.SignalKey) error {
	keyStr := string(key)

	// Try to notify any in-process waiter directly (fast path).
	// Since each key has exactly one waiter, if we find it locally,
	// there's no need to broadcast to the cluster.
	b.mu.RLock()
	ch, exists := b.subscriptions[keyStr]
	b.mu.RUnlock()

	if exists {
		select {
		case ch <- struct{}{}:
			// Successfully notified in-process waiter; no cluster broadcast needed.
			return nil
		default:
			// Channel full - waiter already notified or gone
		}
	}

	// Waiter not in this process; broadcast to cluster via Postgres NOTIFY.
	if b.notifierDriver != nil {
		return b.notifierDriver.Notify(ctx, keyStr)
	}

	return nil
}

func (b *PgxClusterBroadcast) runNotificationLoop(ctx context.Context) {
	const listenTimeout = 30 * time.Second

	for {
		if ctx.Err() != nil {
			return
		}

		timeoutCtx, cancel := context.WithTimeout(ctx, listenTimeout)
		notification, err := b.listenerDriver.WaitForNotification(timeoutCtx)
		cancel()

		if err != nil {
			if ctx.Err() != nil {
				return
			}
			// Log but continue - transient errors shouldn't stop the loop
			if b.logger != nil {
				b.logger.Debugf("notification wait error (will retry): %v", err)
			}
			continue
		}

		if notification != nil {
			key := string(notification.Payload)
			b.mu.RLock()
			ch, exists := b.subscriptions[key]
			b.mu.RUnlock()

			if exists {
				select {
				case ch <- struct{}{}:
				default:
				}
			}
		}
	}
}
