package data

import (
	"context"
	"fmt"

	"gorm.io/gorm"
	gormLogger "gorm.io/gorm/logger"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/project-kessel/inventory-api/internal/data/migrations"
)

var (
	migrationsRun   = migrations.Run
	migrationsRunTo = migrations.RunTo

	// Tests can override this to avoid invoking GORM internals on
	// a real database connection.
	migrationsSession = func(db *gorm.DB) *gorm.DB {
		return db.Session(&gorm.Session{Logger: gormLogger.Default.LogMode(gormLogger.Warn)})
	}
)

func Migrate(db *gorm.DB, logger *log.Helper) error {
	ctx := context.Background()

	if db == nil {
		return fmt.Errorf("cannot migrate with nil db")
	}
	db = migrationsSession(db)
	if err := migrationsRun(ctx, db, logger); err != nil {
		return err
	}
	return nil
}

// Runs all migrations up to and including the specified targetID.
// This is intended for use in tests where engineers need to position the
// database schema at a specific migration before executing further migrations.
func MigrateTo(db *gorm.DB, logger *log.Helper, targetID string) error {
	ctx := context.Background()

	if db == nil {
		return fmt.Errorf("cannot migrate with nil db")
	}
	if targetID == "" {
		return fmt.Errorf("cannot migrate with empty targetID")
	}

	db = migrationsSession(db)
	if err := migrationsRunTo(ctx, db, logger, targetID); err != nil {
		return err
	}
	return nil
}
