package schema

import (
	"github.com/go-gormigrate/gormigrate/v2"
	"gorm.io/gorm"
)

func ReporterResourcesNotTombstoneIdxMigration() *gormigrate.Migration {
	return &gormigrate.Migration{
		ID: "20260326120000",
		Migrate: func(tx *gorm.DB) error {
			if tx.Name() == "postgres" {
				return tx.Exec(`
					CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_reporter_resources_not_tombstone
					ON reporter_resources (resource_type, reporter_type, reporter_instance_id)
					WHERE NOT tombstone
				`).Error
			}
			return tx.Exec(`
				CREATE INDEX IF NOT EXISTS idx_reporter_resources_not_tombstone
				ON reporter_resources (resource_type, reporter_type, reporter_instance_id)
				WHERE NOT tombstone
			`).Error
		},
		Rollback: func(tx *gorm.DB) error {
			return tx.Exec(`DROP INDEX IF EXISTS idx_reporter_resources_not_tombstone`).Error
		},
	}
}
