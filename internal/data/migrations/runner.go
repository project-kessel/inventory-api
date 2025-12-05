package migrations

import (
	"context"
	"fmt"

	"github.com/go-kratos/kratos/v2/log"
	"gorm.io/gorm"
)

type MigrationEngine interface {
	Migrate() error
	MigrateTo(targetID string) error
}

type MigrationEngineFactory interface {
	NewEngine(db *gorm.DB, logger *log.Helper) MigrationEngine
}

var engineFactory MigrationEngineFactory

func init() {
	if engineFactory == nil {
		// Set the default engineFactory to the gormigrate impl
		engineFactory = &gormigrateEngineFactory{}
	}
}

func Run(ctx context.Context, db *gorm.DB, logger *log.Helper) error {
	return runMigrations(ctx, db, logger, func(engine MigrationEngine) error {
		if logger != nil {
			logger.Infof("Starting migrations")
		}

		if err := engine.Migrate(); err != nil {
			if logger != nil {
				logger.Errorf("Migrations failed: %v", err)
			}
			return err
		}

		if logger != nil {
			logger.Info("Migrations completed successfully")
		}

		return nil
	})
}

func RunTo(ctx context.Context, db *gorm.DB, logger *log.Helper, targetID string) error {
	if targetID == "" {
		return fmt.Errorf("cannot run migrations with empty targetID")
	}

	return runMigrations(ctx, db, logger, func(engine MigrationEngine) error {
		if logger != nil {
			logger.Infof("Starting migrations up to %s", targetID)
		}

		if err := engine.MigrateTo(targetID); err != nil {
			if logger != nil {
				logger.Errorf("Migrations up to %s failed: %v", targetID, err)
			}
			return err
		}

		if logger != nil {
			logger.Infof("Migrations up to %s completed successfully", targetID)
		}

		return nil
	})
}

func runMigrations(
	ctx context.Context,
	db *gorm.DB,
	logger *log.Helper,
	migrateFn func(engine MigrationEngine) error,
) error {
	if db == nil {
		return fmt.Errorf("cannot run migrations with nil db")
	}

	return withAdvisoryLock(ctx, db, func(tx *gorm.DB) error {
		engine := engineFactory.NewEngine(tx, logger)
		if err := migrateFn(engine); err != nil {
			return err
		}
		if tx.Name() == "sqlite" {
			if err := tx.Exec(SQLitePragmaForeignKeysOn).Error; err != nil {
				return err
			}
		}
		return nil
	})
}
