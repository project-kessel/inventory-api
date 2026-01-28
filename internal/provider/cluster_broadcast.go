package provider

import (
	"context"
	"fmt"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/project-kessel/inventory-api/internal/biz/model"
	"github.com/project-kessel/inventory-api/internal/data"
	"github.com/project-kessel/inventory-api/internal/pubsub"
)

// NewClusterBroadcast creates a new ClusterBroadcast implementation.
// For postgres databases, this creates a PgxClusterBroadcast that uses
// Postgres LISTEN/NOTIFY for cross-process notification.
// Returns nil if the database type is not postgres.
func NewClusterBroadcast(ctx context.Context, opts *StorageOptions, pgxPool *pgxpool.Pool, logger log.Logger) (model.ClusterBroadcast, error) {
	if opts.Database != DatabasePostgres {
		return nil, nil
	}

	if pgxPool == nil {
		return nil, fmt.Errorf("pgxPool is required for postgres cluster broadcast")
	}

	broadcastLogger := log.NewHelper(log.With(logger, "subsystem", "clusterBroadcast"))

	// Use separate drivers for listening and notifying to avoid mutex contention.
	// The listener driver's connection blocks while waiting for notifications,
	// so a separate notifier driver ensures NOTIFY calls aren't blocked.
	listenerDriver := pubsub.NewPgxDriver(pgxPool)
	notifierDriver := pubsub.NewPgxDriver(pgxPool)

	clusterBroadcast := data.NewPgxClusterBroadcast(data.PgxClusterBroadcastConfig{
		ListenerDriver: listenerDriver,
		NotifierDriver: notifierDriver,
		Logger:         log.With(logger, "subsystem", "clusterBroadcast"),
	})

	if err := clusterBroadcast.Start(ctx); err != nil {
		return nil, fmt.Errorf("error starting cluster broadcast: %w", err)
	}

	broadcastLogger.Info("Cluster broadcast started")

	return clusterBroadcast, nil
}
