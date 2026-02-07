package gorm

import (
	"database/sql"
	"fmt"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/project-kessel/inventory-api/internal/biz"
	"github.com/project-kessel/inventory-api/internal/biz/model"
	"github.com/project-kessel/inventory-api/internal/infrastructure/resourcerepository"
	"github.com/project-kessel/inventory-api/internal/metricscollector"
	"gorm.io/gorm"
)

// PostgresStore implements model.Store, encapsulating persistence concerns.
// It is explicitly a Postgres-backed store.
type PostgresStore struct {
	db                      *gorm.DB
	resourceRepo            resourcerepository.ResourceRepository // existing ResourceRepository for delegation
	eventSource             model.EventSource
	metricsCollector        *metricscollector.MetricsCollector
	maxSerializationRetries int
	logger                  *log.Helper
}

// PostgresStoreConfig holds configuration for creating a PostgresStore.
type PostgresStoreConfig struct {
	DB                      *gorm.DB
	ResourceRepo            resourcerepository.ResourceRepository // existing repository for delegation
	EventSource             model.EventSource
	MetricsCollector        *metricscollector.MetricsCollector
	MaxSerializationRetries int
	Logger                  log.Logger
}

// NewPostgresStore creates a new PostgresStore.
func NewPostgresStore(cfg PostgresStoreConfig) *PostgresStore {
	return &PostgresStore{
		db:                      cfg.DB,
		resourceRepo:            cfg.ResourceRepo,
		eventSource:             cfg.EventSource,
		metricsCollector:        cfg.MetricsCollector,
		maxSerializationRetries: cfg.MaxSerializationRetries,
		logger:                  log.NewHelper(cfg.Logger),
	}
}

// Ensure PostgresStore implements model.Store
var _ model.Store = (*PostgresStore)(nil)

// Begin starts a new serializable transaction.
func (s *PostgresStore) Begin() (model.Tx, error) {
	gormTx := s.db.Begin(&sql.TxOptions{
		Isolation: sql.LevelSerializable,
	})
	if gormTx.Error != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", gormTx.Error)
	}

	return &postgresTx{
		store:  s,
		gormTx: gormTx,
		repo: &txResourceRepository{
			gormTx:   gormTx,
			delegate: s.resourceRepo,
		},
	}, nil
}

// EventSource returns the event source for consuming outbox events.
// May return nil if the store was created without an event source.
func (s *PostgresStore) EventSource() model.EventSource {
	return s.eventSource
}

// postgresTx implements model.Tx
type postgresTx struct {
	store  *PostgresStore
	gormTx *gorm.DB
	repo   *txResourceRepository
	done   bool // true after Commit or Rollback
}

var _ model.Tx = (*postgresTx)(nil)

func (tx *postgresTx) ResourceRepository() model.ResourceRepository {
	return tx.repo
}

func (tx *postgresTx) Commit() error {
	if tx.done {
		return nil
	}
	if err := tx.gormTx.Commit().Error; err != nil {
		return fmt.Errorf("commit failed: %w", err)
	}
	tx.done = true
	return nil
}

// Rollback aborts the transaction. Safe to call after Commit (no-op).
func (tx *postgresTx) Rollback() error {
	if tx.done {
		return nil
	}
	tx.done = true
	return tx.gormTx.Rollback().Error
}

// txResourceRepository implements model.ResourceRepository within a transaction.
// It delegates to the existing ResourceRepository implementation, passing the
// transaction's gorm.DB. This is a bridge pattern for incremental migration.
type txResourceRepository struct {
	gormTx   *gorm.DB
	delegate resourcerepository.ResourceRepository // existing ResourceRepository
}

var _ model.ResourceRepository = (*txResourceRepository)(nil)

func (r *txResourceRepository) NextResourceId() (model.ResourceId, error) {
	return r.delegate.NextResourceId()
}

func (r *txResourceRepository) NextReporterResourceId() (model.ReporterResourceId, error) {
	return r.delegate.NextReporterResourceId()
}

func (r *txResourceRepository) Save(resource model.Resource, operationType biz.EventOperationType, txid string) error {
	return r.delegate.Save(r.gormTx, resource, operationType, txid)
}

func (r *txResourceRepository) FindResourceByKeys(key model.ReporterResourceKey) (*model.Resource, error) {
	return r.delegate.FindResourceByKeys(r.gormTx, key)
}

func (r *txResourceRepository) FindCurrentAndPreviousVersionedRepresentations(key model.ReporterResourceKey, currentVersion *uint, operationType biz.EventOperationType) (*model.Representations, *model.Representations, error) {
	return r.delegate.FindCurrentAndPreviousVersionedRepresentations(r.gormTx, key, currentVersion, operationType)
}

func (r *txResourceRepository) FindLatestRepresentations(key model.ReporterResourceKey) (*model.Representations, error) {
	return r.delegate.FindLatestRepresentations(r.gormTx, key)
}

func (r *txResourceRepository) ContainsEventForTransactionId(transactionId string) (bool, error) {
	return r.delegate.HasTransactionIdBeenProcessed(r.gormTx, transactionId)
}
