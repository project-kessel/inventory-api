package resourcerepository

import (
	bizmodel "github.com/project-kessel/inventory-api/internal/biz/model"
	"gorm.io/gorm"

	"github.com/project-kessel/inventory-api/internal/biz"
)

// TransactionManager provides an abstraction for handling database transactions
// with proper isolation levels and retry mechanisms.
type TransactionManager interface {
	// HandleSerializableTransaction executes the provided function within a serializable transaction.
	// It automatically handles retries in case of serialization failures.
	HandleSerializableTransaction(operationName string, db *gorm.DB, txFunc func(tx *gorm.DB) error) error
}

// ResourceRepository defines the persistence interface for resources.
// This interface is infrastructure-scoped (uses *gorm.DB parameters). The clean
// domain-level interface without gorm coupling lives in [bizmodel.ResourceRepository].
type ResourceRepository interface {
	NextResourceId() (bizmodel.ResourceId, error)
	NextReporterResourceId() (bizmodel.ReporterResourceId, error)
	Save(tx *gorm.DB, resource bizmodel.Resource, operationType biz.EventOperationType, txid string) error
	FindResourceByKeys(tx *gorm.DB, key bizmodel.ReporterResourceKey) (*bizmodel.Resource, error)
	FindCurrentAndPreviousVersionedRepresentations(tx *gorm.DB, key bizmodel.ReporterResourceKey, currentVersion *uint, operationType biz.EventOperationType) (*bizmodel.Representations, *bizmodel.Representations, error)
	FindLatestRepresentations(tx *gorm.DB, key bizmodel.ReporterResourceKey) (*bizmodel.Representations, error)
	GetDB() *gorm.DB
	GetTransactionManager() TransactionManager
	HasTransactionIdBeenProcessed(tx *gorm.DB, transactionId string) (bool, error)
}

// GetCurrentAndPreviousWorkspaceID extracts current and previous workspace IDs from Representations.
func GetCurrentAndPreviousWorkspaceID(current, previous *bizmodel.Representations) (currentWorkspaceID, previousWorkspaceID string) {
	return current.WorkspaceID(), previous.WorkspaceID()
}
