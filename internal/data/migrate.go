package data

import (
	"github.com/go-kratos/kratos/v2/log"
	gormrepo "github.com/project-kessel/inventory-api/internal/infrastructure/resourcerepository/gorm"
	"gorm.io/gorm"
)

// Migrate delegates to [gormrepo.Migrate].
func Migrate(db *gorm.DB, logger *log.Helper) error {
	return gormrepo.Migrate(db, logger)
}

// MigrateTo delegates to [gormrepo.MigrateTo].
func MigrateTo(db *gorm.DB, logger *log.Helper, targetID string) error {
	return gormrepo.MigrateTo(db, logger, targetID)
}
