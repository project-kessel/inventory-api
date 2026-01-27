package data

import (
	"context"
	"sync"

	"github.com/project-kessel/inventory-api/internal/biz/model"
)

// FakeClusterBroadcast implements model.ClusterBroadcast for testing.
// It provides in-memory signal/wait coordination without any external dependencies.
type FakeClusterBroadcast struct {
	mu            sync.RWMutex
	subscriptions map[string]chan struct{}
}

// NewFakeClusterBroadcast creates a new FakeClusterBroadcast for testing.
func NewFakeClusterBroadcast() *FakeClusterBroadcast {
	return &FakeClusterBroadcast{
		subscriptions: make(map[string]chan struct{}),
	}
}

// Ensure FakeClusterBroadcast implements model.ClusterBroadcast
var _ model.ClusterBroadcast = (*FakeClusterBroadcast)(nil)

// Wait blocks until a signal for the given key is received, or the context
// is cancelled.
func (b *FakeClusterBroadcast) Wait(ctx context.Context, key model.SignalKey) error {
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
		return ctx.Err()
	}
}

// Signal broadcasts a signal for the given key to all waiting listeners.
func (b *FakeClusterBroadcast) Signal(ctx context.Context, key model.SignalKey) error {
	keyStr := string(key)

	b.mu.RLock()
	ch, exists := b.subscriptions[keyStr]
	b.mu.RUnlock()

	if exists {
		select {
		case ch <- struct{}{}:
		default:
		}
	}
	return nil
}
