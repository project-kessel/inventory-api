package schema

import (
	"fmt"

	"github.com/go-gormigrate/gormigrate/v2"
	"gorm.io/gorm"
)

// ReporterRepCommonVersionNotNullMigration restores the NOT NULL constraint on
// reporter_representations.common_version. It fails without changing data when
// any existing row has a NULL value.
func ReporterRepCommonVersionNotNullMigration() *gormigrate.Migration {
	return &gormigrate.Migration{
		ID: "20260930180000",
		Migrate: func(tx *gorm.DB) error {
			if err := tx.Exec("ALTER TABLE reporter_representations ALTER COLUMN common_version SET NOT NULL").Error; err != nil { 
				return fmt.Errorf("failed to enforce NOT NULL on reporter_representations.common_version; resolve existing NULL values before retrying: %w", err)
			}
			return nil
		},
		Rollback: func(tx *gorm.DB) error {
			if err := tx.Exec("ALTER TABLE reporter_representations ALTER COLUMN common_version DROP NOT NULL").Error; err != nil {
				return fmt.Errorf("failed to restore nullable reporter_representations.common_version: %w", err)
			}
			return nil
		},
	}
}