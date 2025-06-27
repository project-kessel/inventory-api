package data

import (
	"fmt"

	"gorm.io/gorm"

	"github.com/project-kessel/inventory-api/internal/biz/model"
	"github.com/project-kessel/inventory-api/internal/biz/model/v1beta2"

	"github.com/go-kratos/kratos/v2/log"
)

// Migrate the tables
// See https://gorm.io/docs/migration.html
func Migrate(db *gorm.DB, logger *log.Helper) error {
	models := []interface{}{
		&model.ResourceHistory{},
		&model.Resource{},
		&model.Relationship{},
		&model.RelationshipHistory{},
		&model.LocalInventoryToResource{}, // Deprecated
		&model.InventoryResource{},
		&model.OutboxEvent{},
		// v1beta2 models
		&v1beta2.Resource{},
		&v1beta2.RepresentationReference{},
		&v1beta2.CommonRepresentation{},
		&v1beta2.ReporterRepresentation{},
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
		if gormDbIndexStatement, ok := m.(model.GormDbAfterMigrationHook); ok {
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
