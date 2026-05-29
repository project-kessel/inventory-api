package data

import (
	"database/sql"
	"fmt"

	"github.com/go-kratos/kratos/v2/log"
	"gorm.io/gorm"

	bizmodel "github.com/project-kessel/inventory-api/internal/biz/model"
	"github.com/project-kessel/inventory-api/internal/metricscollector"
)

// GormResourceRepositoryConfig holds configuration for creating a gorm-backed resource repository Store.
type GormResourceRepositoryConfig struct {
	DB                      *gorm.DB
	OutboxPublisher         OutboxPublisher
	MetricsCollector        *metricscollector.MetricsCollector
	MaxSerializationRetries int
}

// gormResourceRepository implements model.Store backed by a GORM database connection.
type gormResourceRepository struct {
	db                      *gorm.DB
	outboxPublisher         OutboxPublisher
	metricsCollector        *metricscollector.MetricsCollector
	maxSerializationRetries int
}

// NewGormResourceRepository creates a new GORM-backed Store.
func NewGormResourceRepository(cfg GormResourceRepositoryConfig) bizmodel.Store {
	publisher := cfg.OutboxPublisher
	if publisher == nil {
		publisher = publishOutboxEvent
	}
	maxRetries := cfg.MaxSerializationRetries
	if maxRetries == 0 {
		maxRetries = 3
	}
	return &gormResourceRepository{
		db:                      cfg.DB,
		outboxPublisher:         publisher,
		metricsCollector:        cfg.MetricsCollector,
		maxSerializationRetries: maxRetries,
	}
}

var _ bizmodel.Store = (*gormResourceRepository)(nil)

// Begin starts a new serializable database transaction.
func (s *gormResourceRepository) Begin() (bizmodel.Tx, error) {
	gormTx := s.db.Begin(&sql.TxOptions{
		Isolation: sql.LevelSerializable,
	})
	if gormTx.Error != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", gormTx.Error)
	}

	return &gormTx_{
		gormTx: gormTx,
		repo: &txResourceRepository{
			gormTx:          gormTx,
			outboxPublisher: s.outboxPublisher,
		},
	}, nil
}

// RunSerializable executes fn within a serializable transaction with automatic
// retry on serialization failures (PostgreSQL 40001 / SQLite busy).
// This preserves the retry behavior from the former HandleSerializableTransaction.
func (s *gormResourceRepository) RunSerializable(operationName string, fn func(tx bizmodel.Tx) error) error {
	var err error
	for i := 0; i < s.maxSerializationRetries; i++ {
		tx, beginErr := s.Begin()
		if beginErr != nil {
			return beginErr
		}

		err = fn(tx)
		if err != nil {
			_ = tx.Rollback()
			if isSerializationFailure(err, i, s.maxSerializationRetries) {
				metricscollector.Incr(s.metricsCollector.SerializationFailures, operationName)
				continue
			}
			return fmt.Errorf("transaction failed: %w", err)
		}

		err = tx.Commit()
		if err != nil {
			_ = tx.Rollback()
			if isSerializationFailure(err, i, s.maxSerializationRetries) {
				metricscollector.Incr(s.metricsCollector.SerializationFailures, operationName)
				continue
			}
			return fmt.Errorf("committing transaction failed: %w", err)
		}
		return nil
	}
	metricscollector.Incr(s.metricsCollector.SerializationExhaustions, operationName)
	log.Errorf("transaction failed after %d attempts: %v", s.maxSerializationRetries, err)
	return fmt.Errorf("transaction failed after %d attempts: %w", s.maxSerializationRetries, err)
}

// gormTx_ implements model.Tx wrapping a GORM transaction.
type gormTx_ struct {
	gormTx *gorm.DB
	repo   *txResourceRepository
	done   bool
}

var _ bizmodel.Tx = (*gormTx_)(nil)

func (tx *gormTx_) ResourceRepository() bizmodel.ResourceRepository {
	return tx.repo
}

func (tx *gormTx_) Commit() error {
	if tx.done {
		return nil
	}
	if err := tx.gormTx.Commit().Error; err != nil {
		return fmt.Errorf("commit failed: %w", err)
	}
	tx.done = true
	return nil
}

func (tx *gormTx_) Rollback() error {
	if tx.done {
		return nil
	}
	tx.done = true
	return tx.gormTx.Rollback().Error
}

// txResourceRepository implements model.ResourceRepository within a transaction.
// It delegates to the package-level functions that perform the actual DB
// operations, passing the transaction's internal gorm.DB handle.
type txResourceRepository struct {
	gormTx          *gorm.DB
	outboxPublisher OutboxPublisher
}

var _ bizmodel.ResourceRepository = (*txResourceRepository)(nil)

func (r *txResourceRepository) NextResourceId() (bizmodel.ResourceId, error) {
	return nextResourceId()
}

func (r *txResourceRepository) NextReporterResourceId() (bizmodel.ReporterResourceId, error) {
	return nextReporterResourceId()
}

func (r *txResourceRepository) Save(resource bizmodel.Resource, operationType bizmodel.EventOperationType, txid bizmodel.TransactionId) error {
	return saveResource(r.gormTx, r.outboxPublisher, resource, operationType, txid)
}

func (r *txResourceRepository) FindResourceByKeys(key bizmodel.ReporterResourceKey) (*bizmodel.Resource, error) {
	return findResourceByKeys(r.gormTx, key)
}

func (r *txResourceRepository) FindCurrentAndPreviousVersionedRepresentations(key bizmodel.ReporterResourceKey, currentVersion *bizmodel.Version, operationType bizmodel.EventOperationType) (*bizmodel.Representations, *bizmodel.Representations, error) {
	return findCurrentAndPreviousVersionedRepresentations(r.gormTx, key, currentVersion, operationType)
}

func (r *txResourceRepository) FindLatestRepresentations(key bizmodel.ReporterResourceKey) (*bizmodel.Representations, error) {
	return findLatestRepresentations(r.gormTx, key)
}

func (r *txResourceRepository) HasTransactionIdBeenProcessed(transactionId bizmodel.TransactionId) (bool, error) {
	return hasTransactionIdBeenProcessed(r.gormTx, transactionId)
}
