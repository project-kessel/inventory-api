package migrations

import (
	"context"
	"fmt"

	"github.com/go-gormigrate/gormigrate/v2"
	"github.com/go-kratos/kratos/v2/log"
	"gorm.io/gorm"
)

func Run(ctx context.Context, db *gorm.DB, logger *log.Helper) error {
	if db == nil {
		return fmt.Errorf("cannot run migrations with nil db")
	}
	options := &gormigrate.Options{
		TableName:                 MigrationTableName,
		IDColumnName:              MigrationIDColumn,
		IDColumnSize:              MigrationIDSize,
		UseTransaction:            false,
		ValidateUnknownMigrations: false,
	}

	return WithAdvisoryLock(ctx, db, MigrationTableName, LockTypeMigrations, func(tx *gorm.DB) error {
		migrationsWithLogs := MigrationsList
		if logger != nil {
			migrationsWithLogs = wrapMigrationsWithLogger(MigrationsList, logger)
		}
		if logger != nil {
			logger.Infof("Starting migrations")
		}
		m := gormigrate.New(tx, options, migrationsWithLogs)
		if err := m.Migrate(); err != nil {
			if logger != nil {
				logger.Errorf("Migrations failed: %v", err)
			}
			return err
		}
		if tx.Dialector.Name() == "sqlite" {
			if err := tx.Exec(SQLitePragmaForeignKeysOn).Error; err != nil {
				return err
			}
		}
		if logger != nil {
			logger.Info("Migrations completed successfully")
		}
		return nil
	})
}

func wrapMigrationsWithLogger(migrations []*gormigrate.Migration, logger *log.Helper) []*gormigrate.Migration {
	wrapped := make([]*gormigrate.Migration, 0, len(migrations))
	for _, mig := range migrations {
		// capture loop variables
		id := mig.ID
		migrateFn := mig.Migrate
		rollbackFn := mig.Rollback

		wrapped = append(wrapped, &gormigrate.Migration{
			ID: id,
			Migrate: func(tx *gorm.DB) error {
				logger.Infof("Applying migration %s", id)
				err := migrateFn(tx)
				if err != nil {
					logger.Errorf("Migration %s failed: %v", id, err)
					return err
				}
				logger.Infof("Applied migration %s", id)
				return nil
			},
			Rollback: func(tx *gorm.DB) error {
				if rollbackFn == nil {
					logger.Infof("No rollback defined for migration %s", id)
					return nil
				}
				logger.Infof("Rolling back migration %s", id)
				err := rollbackFn(tx)
				if err != nil {
					logger.Errorf("Rollback %s failed: %v", id, err)
					return err
				}
				logger.Infof("Rolled back migration %s", id)
				return nil
			},
		})
	}
	return wrapped
}
