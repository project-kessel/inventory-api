package schema

import (
	"github.com/go-gormigrate/gormigrate/v2"
	"gorm.io/gorm"
)

// ReporterRepresentationsHistoryIdxMigration indexes history in generation order.
func ReporterRepresentationsHistoryIdxMigration() *gormigrate.Migration {
	return &gormigrate.Migration{
		ID: "20261005120000",
		Migrate: func(tx *gorm.DB) error {
			if tx.Name() == "postgres" {
				return tx.Exec(`CREATE INDEX CONCURRENTLY reporter_representations_history_idx
					ON reporter_representations (reporter_resource_id, generation DESC, version DESC)`).Error
			}
			return tx.Exec(`CREATE INDEX reporter_representations_history_idx
				ON reporter_representations (reporter_resource_id, generation DESC, version DESC)`).Error
		},
		Rollback: func(tx *gorm.DB) error {
			if tx.Name() == "postgres" {
				return tx.Exec(`DROP INDEX CONCURRENTLY IF EXISTS reporter_representations_history_idx`).Error
			}
			return tx.Exec(`DROP INDEX IF EXISTS reporter_representations_history_idx`).Error
		},
	}
}
