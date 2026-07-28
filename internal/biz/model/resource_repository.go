package model

import "errors"

// ErrSerializationFailure is returned when a transaction fails due to a
// serialization conflict (e.g., PostgreSQL 40001, SQLite busy). Callers
// can check errors.Is(err, ErrSerializationFailure) to decide whether to retry.
var ErrSerializationFailure = errors.New("serialization failure")

// ResourceRepository is the entry point for resource persistence operations.
type ResourceRepository interface {
	// Begin starts a new serializable transaction and returns a ResourceTx.
	// The caller is responsible for calling Commit or Rollback.
	// operationName labels the transaction for metrics. Pass "" for
	// read-only or unlabeled transactions.
	Begin(operationName string) (ResourceTx, error)

	// Transact runs fn inside a serializable transaction with automatic
	// retry on serialization conflicts. The repository handles Begin,
	// Commit, Rollback, retry, and exhaustion metrics internally.
	Transact(operationName string, fn func(ResourceTx) error) error
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
