package schema

import (
	"gorm.io/gorm"

	"github.com/go-gormigrate/gormigrate/v2"
)

// ReporterRepCommonVersionNullable makes the common_version column nullable in reporter_representations
// to support reporter-only resources (resources that never have common representation data).
func ReporterRepCommonVersionNullable() *gormigrate.Migration {
	return &gormigrate.Migration{
		ID: "20260910120000",
		Migrate: func(tx *gorm.DB) error {
			// Make common_version nullable
			return tx.Exec("ALTER TABLE reporter_representations ALTER COLUMN common_version DROP NOT NULL").Error
		},
		Rollback: func(tx *gorm.DB) error {
			// Rollback: make common_version NOT NULL again
			// This will fail if there are any NULL values in the column
			return tx.Exec("ALTER TABLE reporter_representations ALTER COLUMN common_version SET NOT NULL").Error
		},
	}
}
