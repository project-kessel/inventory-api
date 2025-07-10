package data

import (
	"fmt"

	"github.com/project-kessel/inventory-api/internal/biz/model"
	"gorm.io/gorm"

	"github.com/project-kessel/inventory-api/internal/biz/model_legacy"

	"github.com/go-kratos/kratos/v2/log"
)

// Migrate the tables
// See https://gorm.io/docs/migration.html
func Migrate(db *gorm.DB, logger *log.Helper) error {
	models := []interface{}{
		&model_legacy.ResourceHistory{},
		&model_legacy.Resource{},
		&model_legacy.Relationship{},
		&model_legacy.RelationshipHistory{},
		&model_legacy.LocalInventoryToResource{}, // Deprecated
		&model_legacy.InventoryResource{},
		&model_legacy.OutboxEvent{},
		&model.ReporterRepresentation{},
		&model.CommonRepresentation{},
	}

	if err := db.AutoMigrate(models...); err != nil {
		return fmt.Errorf("auto migration has failed: %w", err)
	}

	if db.Name() == "sqlite" {
		// Ensures sqlite honors the foreign keys
		err := db.Exec("PRAGMA foreign_keys = ON").Error
		if err != nil {
			return err
		}
	}

	for _, m := range models {
		if gormDbIndexStatement, ok := m.(model_legacy.GormDbAfterMigrationHook); ok {
			statement := &gorm.Statement{DB: db}
			err := statement.Parse(m)
			if err != nil {
				return fmt.Errorf("statement parsing has failed: %w", err)
			}

			err = gormDbIndexStatement.GormDbAfterMigration(db, statement.Schema)
			if err != nil {
				return fmt.Errorf("migration failure: %w", err)
			}
		}
	}

	logger.Info("Migration successful!")
	return nil
}
