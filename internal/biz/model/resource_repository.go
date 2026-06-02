package model

// ResourceRepository is the entry point for resource persistence operations.
// All data access goes through Transact, which provides a transaction-scoped ResourceTx.
type ResourceRepository interface {
	// Transact executes fn within a serializable transaction. If fn returns nil,
	// the transaction commits. If fn returns an error, it rolls back.
	// Serialization failures are retried automatically.
	Transact(fn func(tx ResourceTx) error) error
}

// ResourceTx provides resource operations scoped to a serializable transaction.
// Commit and rollback are managed automatically by Transact.
type ResourceTx interface {
	NextResourceId() (ResourceId, error)
	NextReporterResourceId() (ReporterResourceId, error)
	Save(resource Resource, operationType EventOperationType, txid TransactionId) error
	FindResourceByKeys(key ReporterResourceKey) (*Resource, error)
	FindCurrentAndPreviousVersionedRepresentations(key ReporterResourceKey, currentVersion *Version, operationType EventOperationType) (*Representations, *Representations, error)
	FindLatestRepresentations(key ReporterResourceKey) (*Representations, error)
	HasTransactionIdBeenProcessed(transactionId TransactionId) (bool, error)
}
