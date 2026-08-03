package schema

import (
	"github.com/go-gormigrate/gormigrate/v2"
	"gorm.io/gorm"
)

func DropOutboxEventsMigration() *gormigrate.Migration {
	return &gormigrate.Migration{
		ID: "20260803120000",
		Migrate: func(tx *gorm.DB) error {
			return tx.Exec("DROP TABLE IF EXISTS outbox_events").Error
		},
		Rollback: func(tx *gorm.DB) error {
			return nil
		},
	}
}
