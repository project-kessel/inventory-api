package model

// Store provides transactional access to resource persistence.
type Store interface {
	// Begin starts a new unit of work.
	Begin() (Tx, error)

	// RunSerializable executes fn within a serializable transaction with
	// automatic retry on serialization failures. The Tx passed to fn must
	// not be committed or rolled back by the caller -- RunSerializable
	// handles the lifecycle.
	RunSerializable(operationName string, fn func(tx Tx) error) error
}

// Tx represents a database transaction with access to repositories.
type Tx interface {
	// ResourceRepository returns the repository for resource operations
	// within this transaction.
	ResourceRepository() ResourceRepository

	// Commit commits the transaction.
	Commit() error

	// Rollback aborts the transaction. It is safe to call after Commit
	// (it will be a no-op), allowing the pattern: defer tx.Rollback()
	Rollback() error
}

// ResourceRepository defines the persistence interface for resources.
// All operations execute within the transaction scope provided by the
// enclosing Tx -- callers never pass a transaction handle.
type ResourceRepository interface {
	NextResourceId() (ResourceId, error)
	NextReporterResourceId() (ReporterResourceId, error)
	Save(resource Resource, operationType EventOperationType, txid TransactionId) error
	FindResourceByKeys(key ReporterResourceKey) (*Resource, error)
	FindCurrentAndPreviousVersionedRepresentations(key ReporterResourceKey, currentVersion *Version, operationType EventOperationType) (*Representations, *Representations, error)
	FindLatestRepresentations(key ReporterResourceKey) (*Representations, error)
	HasTransactionIdBeenProcessed(transactionId TransactionId) (bool, error)
}
