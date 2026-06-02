package model

import "errors"

// ErrSerializationFailure is returned when a transaction fails due to a
// serialization conflict (e.g., PostgreSQL 40001, SQLite busy). Callers
// can check errors.Is(err, ErrSerializationFailure) to decide whether to retry.
var ErrSerializationFailure = errors.New("serialization failure")

// ResourceRepository is the entry point for resource persistence operations.
// Callers obtain a ResourceTx via Begin and manage its lifecycle explicitly.
type ResourceRepository interface {
	// Begin starts a new serializable transaction and returns a ResourceTx.
	Begin() (ResourceTx, error)

	// MaxSerializationRetries returns the configured maximum number of
	// retry attempts for serialization failures.
	MaxSerializationRetries() int

	// RecordSerializationExhaustion records a metric when all retry attempts
	// are exhausted. Called by the caller after the retry loop exits.
	RecordSerializationExhaustion()
}

// ResourceTx provides resource operations scoped to a serializable transaction.
// The caller is responsible for calling Commit or Rollback.
type ResourceTx interface {
	NextResourceId() (ResourceId, error)
	NextReporterResourceId() (ReporterResourceId, error)
	Save(resource Resource, operationType EventOperationType, txid TransactionId) error
	FindResourceByKeys(key ReporterResourceKey) (*Resource, error)
	FindCurrentAndPreviousVersionedRepresentations(key ReporterResourceKey, currentVersion *Version, operationType EventOperationType) (*Representations, *Representations, error)
	FindLatestRepresentations(key ReporterResourceKey) (*Representations, error)
	HasTransactionIdBeenProcessed(transactionId TransactionId) (bool, error)
	Commit() error
	Rollback() error
}
