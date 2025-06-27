package v1beta2

import (
	"context"

	"gorm.io/gorm"
)

// isInTransaction checks if the database connection is already in a transaction
func isInTransaction(db *gorm.DB) bool {
	// In GORM v2, we can check if we're in a transaction by looking at the InstanceSet
	// When in a transaction, GORM sets a "gorm:started_transaction" key
	_, inTx := db.InstanceGet("gorm:started_transaction")
	return inTx
}

// WithTx runs fn inside an existing transaction or starts a new one.
// If the provided db is already a transaction, it will use it; otherwise it will create a new transaction.
func WithTx(ctx context.Context, db *gorm.DB, fn func(*gorm.DB) error) error {
	if isInTransaction(db) {
		return fn(db.WithContext(ctx))
	}
	return db.WithContext(ctx).Transaction(fn)
}
