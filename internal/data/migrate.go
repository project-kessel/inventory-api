package data

import (
	"context"
	"fmt"

	"gorm.io/gorm"
	gormLogger "gorm.io/gorm/logger"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/project-kessel/inventory-api/internal/data/migrations"
)

func Migrate(db *gorm.DB, logger *log.Helper) error {
	ctx := context.Background()

	if db == nil {
		return fmt.Errorf("cannot migrate with nil db")
	}
	// Ensure GORM logs at least WARN during migrations so problematic SQL statements are visible
	_ = db.Session(&gorm.Session{Logger: gormLogger.Default.LogMode(gormLogger.Warn)})
	if err := migrations.Run(ctx, db, logger); err != nil {
		return err
	}
	return nil
}
