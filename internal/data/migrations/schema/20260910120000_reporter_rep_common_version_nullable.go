package schema

import (
	"fmt"

	"github.com/go-gormigrate/gormigrate/v2"
	"gorm.io/gorm"
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
			return fmt.Errorf("irreversible migration: common_version has been made nullable to support reporter-only resources, and the database may contain legitimate NULL values that cannot be safely converted back to NOT NULL")
		},
	}
}
