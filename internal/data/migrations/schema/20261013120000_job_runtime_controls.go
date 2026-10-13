package schema

import (
	"github.com/go-gormigrate/gormigrate/v2"
	"gorm.io/gorm"
)

// JobRuntimeControlsMigration creates the table that lets operators pause,
// resume and stop the retention cleanup jobs while they run in production
// (a single SQL UPDATE; no pod access needed). The cleanup jobs tolerate the
// table being absent — controls are disabled with a warning in that case, so
// deploy ordering is never a hard dependency.
func JobRuntimeControlsMigration() *gormigrate.Migration {
	return &gormigrate.Migration{
		ID: "20261013120000",
		Migrate: func(tx *gorm.DB) error {
			return tx.Exec(`
				CREATE TABLE IF NOT EXISTS job_runtime_controls (
					job_name VARCHAR(128) PRIMARY KEY,
					desired_state VARCHAR(16) NOT NULL CHECK (desired_state IN ('running', 'paused', 'stop')),
					updated_by VARCHAR(255) NOT NULL DEFAULT '',
					note TEXT NOT NULL DEFAULT '',
					updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
				)
			`).Error
		},
		Rollback: func(tx *gorm.DB) error {
			return tx.Exec(`DROP TABLE IF EXISTS job_runtime_controls`).Error
		},
	}
}
