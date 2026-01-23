package data

import (
	"context"
	"fmt"
	"sync"

	"github.com/project-kessel/inventory-api/internal/biz"
	"github.com/project-kessel/inventory-api/internal/biz/model"
	"github.com/project-kessel/inventory-api/internal/pubsub"
	"gorm.io/gorm"
)

// AdapterStore adapts the old ResourceRepository to the new model.Store interface.
// This is a temporary bridge to allow incremental migration.
type AdapterStore struct {
	resourceRepo  ResourceRepository
	listenManager pubsub.ListenManagerImpl

	// Event channel (not used in adapter - events come from consumer)
	events chan model.OutboxEvent
}

// NewAdapterStore creates a new AdapterStore.
func NewAdapterStore(resourceRepo ResourceRepository, listenManager pubsub.ListenManagerImpl) *AdapterStore {
	return &AdapterStore{
		resourceRepo:  resourceRepo,
		listenManager: listenManager,
		events:        make(chan model.OutboxEvent, 100),
	}
}

var _ model.Store = (*AdapterStore)(nil)

// Begin starts a new transaction.
func (s *AdapterStore) Begin() (model.Tx, error) {
	gormDB := s.resourceRepo.GetDB()
	gormTx := gormDB.Begin()
	if gormTx.Error != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", gormTx.Error)
	}

	return &adapterTx{
		gormTx:       gormTx,
		resourceRepo: s.resourceRepo,
	}, nil
}

// Events returns the event channel.
func (s *AdapterStore) Events() <-chan model.OutboxEvent {
	return s.events
}

// WaitForReplication waits for replication to complete.
// Uses the ListenManager for backward compatibility.
func (s *AdapterStore) WaitForReplication(ctx context.Context, txid string) error {
	if s.listenManager == nil {
		return nil
	}

	subscription := s.listenManager.Subscribe(txid)
	defer subscription.Unsubscribe()

	return subscription.BlockForNotification(ctx)
}

// NotifyReplicationComplete notifies that replication is complete.
func (s *AdapterStore) NotifyReplicationComplete(ctx context.Context, txid string) error {
	// Not used in adapter mode - the consumer handles this
	return nil
}

// adapterTx implements model.Tx by wrapping a gorm transaction.
type adapterTx struct {
	gormTx       *gorm.DB
	resourceRepo ResourceRepository
	mu           sync.Mutex
	done         bool // true after Commit or Rollback
}

var _ model.Tx = (*adapterTx)(nil)

func (tx *adapterTx) ResourceRepository() model.ResourceRepository {
	return &adapterResourceRepository{
		gormTx:       tx.gormTx,
		resourceRepo: tx.resourceRepo,
	}
}

func (tx *adapterTx) Commit() error {
	tx.mu.Lock()
	defer tx.mu.Unlock()

	if tx.done {
		return nil
	}

	err := tx.gormTx.Commit().Error
	if err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}
	tx.done = true
	return nil
}

// Rollback aborts the transaction. Safe to call after Commit (no-op).
func (tx *adapterTx) Rollback() error {
	tx.mu.Lock()
	defer tx.mu.Unlock()

	if tx.done {
		return nil
	}
	tx.done = true

	return tx.gormTx.Rollback().Error
}

// adapterResourceRepository adapts ResourceRepository to model.ResourceRepository.
type adapterResourceRepository struct {
	gormTx       *gorm.DB
	resourceRepo ResourceRepository
}

var _ model.ResourceRepository = (*adapterResourceRepository)(nil)

func (r *adapterResourceRepository) NextResourceId() (model.ResourceId, error) {
	return r.resourceRepo.NextResourceId()
}

func (r *adapterResourceRepository) NextReporterResourceId() (model.ReporterResourceId, error) {
	return r.resourceRepo.NextReporterResourceId()
}

func (r *adapterResourceRepository) Save(resource model.Resource, operationType biz.EventOperationType, txid string) error {
	return r.resourceRepo.Save(r.gormTx, resource, operationType, txid)
}

func (r *adapterResourceRepository) FindResourceByKeys(key model.ReporterResourceKey) (*model.Resource, error) {
	return r.resourceRepo.FindResourceByKeys(r.gormTx, key)
}

func (r *adapterResourceRepository) FindCurrentAndPreviousVersionedRepresentations(key model.ReporterResourceKey, currentVersion *uint, operationType biz.EventOperationType) (*model.Representations, *model.Representations, error) {
	return r.resourceRepo.FindCurrentAndPreviousVersionedRepresentations(r.gormTx, key, currentVersion, operationType)
}

func (r *adapterResourceRepository) FindLatestRepresentations(key model.ReporterResourceKey) (*model.Representations, error) {
	return r.resourceRepo.FindLatestRepresentations(r.gormTx, key)
}

func (r *adapterResourceRepository) ContainsEventForTransactionId(transactionId string) (bool, error) {
	return r.resourceRepo.HasTransactionIdBeenProcessed(r.gormTx, transactionId)
}
