// Package replication provides the usecase for replicating inventory changes to the relations service.
package replication

import (
	"context"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/project-kessel/inventory-api/internal/biz/model"
)

// RelationReplicationUsecase orchestrates the replication of inventory events to relations.
// It consumes events from the Store and coordinates the transaction, replication, and notification.
type RelationReplicationUsecase struct {
	store              model.Store
	replicationService *model.RelationReplicationService
	logger             *log.Helper
}

// NewRelationReplicationUsecase creates a new RelationReplicationUsecase.
func NewRelationReplicationUsecase(
	store model.Store,
	replicationService *model.RelationReplicationService,
	logger log.Logger,
) *RelationReplicationUsecase {
	return &RelationReplicationUsecase{
		store:              store,
		replicationService: replicationService,
		logger:             log.NewHelper(logger),
	}
}

// Run processes events from the store's event channel until the context is cancelled.
func (u *RelationReplicationUsecase) Run(ctx context.Context) error {
	u.logger.Info("Starting relation replication usecase")

	for {
		select {
		case <-ctx.Done():
			u.logger.Info("Relation replication usecase stopped")
			return ctx.Err()
		case event, ok := <-u.store.Events():
			if !ok {
				u.logger.Info("Event channel closed")
				return nil
			}
			u.handleEvent(ctx, event)
		}
	}
}

func (u *RelationReplicationUsecase) handleEvent(ctx context.Context, event model.OutboxEvent) {
	txid := event.TxID().String()
	u.logger.Infof("Processing replication event: operation=%s, txid=%s", event.Operation(), txid)

	tx, err := u.store.Begin()
	if err != nil {
		u.logger.Errorf("Failed to begin transaction: %v", err)
		return
	}
	defer tx.Rollback()

	err = u.replicationService.Replicate(ctx, tx, event)
	if err != nil {
		u.logger.Errorf("Failed to replicate event: %v", err)
		return
	}

	if err := tx.Commit(); err != nil {
		u.logger.Errorf("Failed to commit transaction: %v", err)
		return
	}

	// Notify that replication is complete for this txid
	if txid != "" {
		if err := u.store.NotifyReplicationComplete(ctx, txid); err != nil {
			u.logger.Errorf("Failed to notify replication complete: %v", err)
			// Don't return - the replication succeeded, just notification failed
		} else {
			u.logger.Debugf("Notified replication complete for txid=%s", txid)
		}
	}
}
