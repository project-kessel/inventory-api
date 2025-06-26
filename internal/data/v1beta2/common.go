package v1beta2

import "gorm.io/gorm"

// isInTransaction checks if the database connection is already in a transaction
func isInTransaction(db *gorm.DB) bool {
	// In GORM v2, we can check if we're in a transaction by looking at the InstanceSet
	// When in a transaction, GORM sets a "gorm:started_transaction" key
	_, inTx := db.InstanceGet("gorm:started_transaction")
	return inTx
}
