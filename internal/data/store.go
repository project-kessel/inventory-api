package data

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/jackc/pgx/v5/pgxpool"
	"gorm.io/gorm"

	"github.com/project-kessel/inventory-api/internal/biz"
	"github.com/project-kessel/inventory-api/internal/biz/model"
	"github.com/project-kessel/inventory-api/internal/metricscollector"
	"github.com/project-kessel/inventory-api/internal/pubsub"
)

// EventSource provides events to the Store. The Store owns and runs the
// EventSource, which encapsulates how events are received (e.g., from Kafka).
type EventSource interface {
	// Run starts the event source and calls emit for each event received.
	// It blocks until ctx is cancelled or an error occurs.
	Run(ctx context.Context, emit func(model.OutboxEvent)) error
}

// PostgresStore implements model.Store, encapsulating all persistence concerns
// including transactions, event streaming, and replication notifications.
// It is explicitly a Postgres-backed store with replication to Kafka.
type PostgresStore struct {
	db                      *gorm.DB
	pgPool                  *pgxpool.Pool
	resourceRepo            ResourceRepository // existing data.ResourceRepository for delegation
	metricsCollector        *metricscollector.MetricsCollector
	maxSerializationRetries int
	logger                  *log.Helper

	// Event source (e.g., Kafka consumer) - owned by this Store
	eventSource EventSource

	// Event channel for outbox events (fed by eventSource)
	events chan model.OutboxEvent

	// Notification management (encapsulates LISTEN/NOTIFY)
	notifyDriver  pubsub.Driver
	mu            sync.RWMutex
	subscriptions map[string]chan struct{}
}

// PostgresStoreConfig holds configuration for creating a PostgresStore.
type PostgresStoreConfig struct {
	DB                      *gorm.DB
	PgPool                  *pgxpool.Pool
	ResourceRepo            ResourceRepository // existing repository for delegation
	EventSource             EventSource        // Kafka consumer or other event source (owned by Store)
	MetricsCollector        *metricscollector.MetricsCollector
	MaxSerializationRetries int
	Logger                  log.Logger
	EventBufferSize         int
}

// NewPostgresStore creates a new PostgresStore.
func NewPostgresStore(cfg PostgresStoreConfig) *PostgresStore {
	bufferSize := cfg.EventBufferSize
	if bufferSize == 0 {
		bufferSize = 100
	}

	var driver pubsub.Driver
	if cfg.PgPool != nil {
		driver = pubsub.NewPgxDriver(cfg.PgPool)
	}

	return &PostgresStore{
		db:                      cfg.DB,
		pgPool:                  cfg.PgPool,
		resourceRepo:            cfg.ResourceRepo,
		eventSource:             cfg.EventSource,
		metricsCollector:        cfg.MetricsCollector,
		maxSerializationRetries: cfg.MaxSerializationRetries,
		logger:                  log.NewHelper(cfg.Logger),
		events:                  make(chan model.OutboxEvent, bufferSize),
		notifyDriver:            driver,
		subscriptions:           make(map[string]chan struct{}),
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

// Events returns the channel of OutboxEvents.
// This encapsulates the Kafka consumer - the Store internally consumes from Kafka
// (via Debezium CDC reading the outbox) and emits events to this channel.
// The domain layer consumes from this channel to handle replication without
// knowing about Kafka/Debezium implementation details.
func (s *PostgresStore) Events() <-chan model.OutboxEvent {
	return s.events
}

// emitEvent sends an event to the Events channel. This is called by the
// internal event source (Kafka consumer) when a message is received.
func (s *PostgresStore) emitEvent(event model.OutboxEvent) {
	select {
	case s.events <- event:
	default:
		s.logger.Warnf("event channel full, dropping event for txid %s", event.TxID())
	}
}

// WaitForReplication blocks until replication for the given txid completes.
// This works both in-process (via NotifyReplicationComplete in the same process)
// and cross-process (via Postgres LISTEN/NOTIFY). For cross-process notification,
// StartNotificationListener must be called at startup.
func (s *PostgresStore) WaitForReplication(ctx context.Context, txid string) error {
	// Create a subscription channel for this txid.
	// The channel will be signaled either by:
	// 1. In-process: NotifyReplicationComplete directly signals the channel
	// 2. Cross-process: runNotificationLoop receives Postgres NOTIFY and signals
	ch := make(chan struct{}, 1)

	s.mu.Lock()
	s.subscriptions[txid] = ch
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		delete(s.subscriptions, txid)
		s.mu.Unlock()
	}()

	select {
	case <-ch:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("wait for replication cancelled: %w", ctx.Err())
	}
}

// NotifyReplicationComplete signals that replication for txid has completed.
// This notifies both in-process waiters (directly via channel) and cross-process
// waiters (via Postgres NOTIFY). The consumer calls this after processing an event.
func (s *PostgresStore) NotifyReplicationComplete(ctx context.Context, txid string) error {
	// First, notify any in-process waiters directly (fast path)
	s.mu.RLock()
	ch, exists := s.subscriptions[txid]
	s.mu.RUnlock()

	if exists {
		select {
		case ch <- struct{}{}:
		default:
			// Channel full or closed, that's ok
		}
	}

	// Also send via Postgres NOTIFY for cross-process notification.
	// This allows waiters in other processes to be notified.
	if s.notifyDriver != nil {
		return s.notifyDriver.Notify(ctx, txid)
	}

	return nil
}

// Start starts the Store's background processes:
// - The event source (Kafka consumer) which feeds Events()
// - The notification listener (Postgres LISTEN) for cross-process replication notifications
// This should be called once at application startup.
func (s *PostgresStore) Start(ctx context.Context) error {
	// Start notification listener for cross-process LISTEN/NOTIFY
	if s.notifyDriver != nil {
		if err := s.notifyDriver.Connect(ctx); err != nil {
			return fmt.Errorf("failed to connect notification driver: %w", err)
		}

		if err := s.notifyDriver.Listen(ctx); err != nil {
			return fmt.Errorf("failed to start listening: %w", err)
		}

		go s.runNotificationLoop(ctx)
	}

	// Start event source (Kafka consumer) - it feeds emitEvent
	if s.eventSource != nil {
		go func() {
			if err := s.eventSource.Run(ctx, s.emitEvent); err != nil {
				s.logger.Errorf("event source error: %v", err)
			}
		}()
	}

	return nil
}

func (s *PostgresStore) runNotificationLoop(ctx context.Context) {
	const listenTimeout = 30 * time.Second

	for {
		if ctx.Err() != nil {
			return
		}

		timeoutCtx, cancel := context.WithTimeout(ctx, listenTimeout)
		notification, err := s.notifyDriver.WaitForNotification(timeoutCtx)
		cancel()

		if err != nil {
			if ctx.Err() != nil {
				return
			}
			// Log but continue - transient errors shouldn't stop the loop
			s.logger.Debugf("notification wait error (will retry): %v", err)
			continue
		}

		if notification != nil {
			txid := string(notification.Payload)
			s.mu.RLock()
			ch, exists := s.subscriptions[txid]
			s.mu.RUnlock()

			if exists {
				select {
				case ch <- struct{}{}:
				default:
				}
			}
		}
	}
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
	delegate ResourceRepository // existing data.ResourceRepository
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
