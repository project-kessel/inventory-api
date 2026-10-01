package schema

import (
	"github.com/go-gormigrate/gormigrate/v2"
	"gorm.io/gorm"
)

// RetentionCleanupIndexesMigration adds indexes on created_at columns for reporter_representations
// and common_representations tables to support efficient time-based retention cleanup queries.
func RetentionCleanupIndexesMigration() *gormigrate.Migration {
	return &gormigrate.Migration{
		ID: "20261001120000",
		Migrate: func(tx *gorm.DB) error {
			// For PostgreSQL, use CONCURRENTLY to avoid blocking production traffic
			if tx.Name() == "postgres" {
				// Create index on reporter_representations.created_at
				if err := tx.Exec(`
					CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_reporter_reps_created_at
					ON reporter_representations (created_at)
				`).Error; err != nil {
					return err
				}

				// Create index on common_representations.created_at
				if err := tx.Exec(`
					CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_common_reps_created_at
					ON common_representations (created_at)
				`).Error; err != nil {
					return err
				}

				return nil
			}

			// For other databases (SQLite), create indexes without CONCURRENTLY
			if err := tx.Exec(`
				CREATE INDEX IF NOT EXISTS idx_reporter_reps_created_at
				ON reporter_representations (created_at)
			`).Error; err != nil {
				return err
			}

			if err := tx.Exec(`
				CREATE INDEX IF NOT EXISTS idx_common_reps_created_at
				ON common_representations (created_at)
			`).Error; err != nil {
				return err
			}

			return nil
		},
		Rollback: func(tx *gorm.DB) error {
			// Drop reporter_representations index
			if err := tx.Exec(`DROP INDEX IF EXISTS idx_reporter_reps_created_at`).Error; err != nil {
				return err
			}

			// Drop common_representations index
			if err := tx.Exec(`DROP INDEX IF EXISTS idx_common_reps_created_at`).Error; err != nil {
				return err
			}

			return nil
		},
	}
}
