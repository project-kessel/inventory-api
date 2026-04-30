package model

import (
	"gorm.io/gorm"
)

type ResourceRepository interface {
	NextResourceId() (ResourceId, error)
	NextReporterResourceId() (ReporterResourceId, error)
	Save(tx *gorm.DB, resource Resource, operationType EventOperationType, txid TransactionId) error
	FindResourceByKeys(tx *gorm.DB, key ReporterResourceKey) (*Resource, error)
	FindCurrentAndPreviousVersionedRepresentations(tx *gorm.DB, key ReporterResourceKey, currentVersion *Version, operationType EventOperationType) (*Representations, *Representations, error)
	FindLatestRepresentations(tx *gorm.DB, key ReporterResourceKey) (*Representations, error)
	GetDB() *gorm.DB
	GetTransactionManager() TransactionManager
	HasTransactionIdBeenProcessed(tx *gorm.DB, transactionId TransactionId) (bool, error)
}
