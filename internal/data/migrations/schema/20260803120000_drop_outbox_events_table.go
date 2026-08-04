package schema

import (
	"fmt"

	"github.com/go-gormigrate/gormigrate/v2"
	"gorm.io/gorm"
)

func DropOutboxEventsMigration() *gormigrate.Migration {
	return &gormigrate.Migration{
		ID: "20260803120000",
		Migrate: func(tx *gorm.DB) error {
			if err := tx.Exec("DROP TABLE IF EXISTS outbox_events").Error; err != nil {
				return fmt.Errorf("failed to drop outbox_events table: %w", err)
			}
			return nil
		},
		Rollback: func(tx *gorm.DB) error {
			return fmt.Errorf("irreversible migration: outbox_events table has been dropped and cannot be restored")
		},
	}
}
