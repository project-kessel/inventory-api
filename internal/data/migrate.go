package data

import (
	"github.com/project-kessel/inventory-api/internal/biz/model"
	"gorm.io/gorm"

	"github.com/go-kratos/kratos/v2/log"
)

// Migrate the tables
// See https://gorm.io/docs/migration.html
func Migrate(db *gorm.DB, logger *log.Helper) error {
	if err := db.AutoMigrate(
		&model.ResourceHistory{},
		&model.Resource{},
		&model.Relationship{},
		&model.RelationshipHistory{},
		&model.LocalInventoryToResource{},
	); err != nil {
		return err
	}
	logger.Info("Migration successful!")
	return nil
}
