// Package replication provides the usecase for replicating inventory changes to the relations service.
package replication

import (
	"context"
	"fmt"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/project-kessel/inventory-api/internal/biz/model"
)

// RelationReplicationUsecase orchestrates the replication of inventory events to relations.
// It consumes events from the Store's EventSource and replicates them to the relations service.
type RelationReplicationUsecase struct {
	store              model.Store
	clusterBroadcast   model.ClusterBroadcast
	replicationService *model.RelationReplicationService
	logger             *log.Helper
}

// NewRelationReplicationUsecase creates a new RelationReplicationUsecase.
func NewRelationReplicationUsecase(
	store model.Store,
	clusterBroadcast model.ClusterBroadcast,
	replicationService *model.RelationReplicationService,
	logger log.Logger,
) *RelationReplicationUsecase {
	return &RelationReplicationUsecase{
		store:              store,
		clusterBroadcast:   clusterBroadcast,
		replicationService: replicationService,
		logger:             log.NewHelper(logger),
	}
}

// Run starts consuming events from the Store's EventSource and processing them.
// It blocks until the context is cancelled or a fatal error occurs.
// Returns nil if the Store has no EventSource configured.
func (u *RelationReplicationUsecase) Run(ctx context.Context) error {
	eventSource := u.store.EventSource()
	if eventSource == nil {
		u.logger.Warn("No event source configured, replication usecase will not run")
		return nil
	}

	u.logger.Info("Starting relation replication usecase")
	return eventSource.Run(ctx, u.handle)
}

// handle processes a single delivery synchronously.
// Returns nil on success, ErrFencingFailed for safe redelivery, or other error for fatal failure.
func (u *RelationReplicationUsecase) handle(ctx context.Context, delivery model.Delivery) error {
	event := delivery.Event()
	txid := event.TxID().String()
	u.logger.Infof("Processing replication event: operation=%s, txid=%s", event.Operation(), txid)

	tx, err := u.store.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	err = u.replicationService.Replicate(ctx, tx, delivery)
	if err != nil {
		// ErrFencingFailed will be detected by the caller for safe redelivery
		return fmt.Errorf("failed to replicate: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	// Signal that replication is complete for this txid
	if txid != "" && u.clusterBroadcast != nil {
		if err := u.clusterBroadcast.Signal(ctx, model.SignalKey(txid)); err != nil {
			u.logger.Errorf("Failed to signal replication complete: %v", err)
			// Don't return error - the replication succeeded, just notification failed
		} else {
			u.logger.Debugf("Signaled replication complete for txid=%s", txid)
		}
	}

	return nil
}
