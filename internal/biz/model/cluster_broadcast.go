package model

import "context"

// SignalKey is a type alias for broadcast signal identifiers.
type SignalKey string

// ClusterBroadcast provides cluster-wide signal broadcasting capabilities.
// It enables coordination between cluster nodes without being tied to any
// specific use case (e.g., read-after-write consistency, cache invalidation).
type ClusterBroadcast interface {
	// Wait blocks until a signal for the given key is received,
	// or the context is cancelled.
	Wait(ctx context.Context, key SignalKey) error

	// Signal broadcasts a signal for the given key to all waiting
	// listeners across the cluster.
	Signal(ctx context.Context, key SignalKey) error
}
