package schema

import (
	"github.com/go-gormigrate/gormigrate/v2"
	"gorm.io/gorm"
)

// ReporterRepsGenVerIdxMigration adds a composite index on reporter_representations
// for (reporter_resource_id, generation DESC, version DESC).
//
// The existing primary key is (reporter_resource_id, version, generation) which does not
// support the ORDER BY generation DESC, version DESC pattern used by multiple queries
// (fetchPreviousReporterRepresentation, fetchLatestReporterRepresentation,
// fetchLastLiveReporterBefore). At scale (33M+ rows) this forces PostgreSQL to fetch
// all rows for a reporter_resource_id and sort them, making these queries slow.
//
// This index enables a backward index scan with LIMIT 1, reducing those queries to
// a single index seek.
func ReporterRepsGenVerIdxMigration() *gormigrate.Migration {
	return &gormigrate.Migration{
		ID: "20260930160000",
		Migrate: func(tx *gorm.DB) error {
			if tx.Name() == "postgres" {
				return tx.Exec(`
					CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_reporter_reps_resource_gen_ver
					ON reporter_representations (reporter_resource_id, generation DESC, version DESC)
				`).Error
			}
			return tx.Exec(`
				CREATE INDEX IF NOT EXISTS idx_reporter_reps_resource_gen_ver
				ON reporter_representations (reporter_resource_id, generation DESC, version DESC)
			`).Error
		},
		Rollback: func(tx *gorm.DB) error {
			return tx.Exec(`DROP INDEX IF EXISTS idx_reporter_reps_resource_gen_ver`).Error
		},
	}
}
