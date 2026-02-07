package migrations

import (
	"github.com/go-gormigrate/gormigrate/v2"
	"github.com/go-kratos/kratos/v2/log"
	"gorm.io/gorm"
)

var (
	_ MigrationEngineFactory = (*gormigrateEngineFactory)(nil)
	_ MigrationEngine        = (*gormigrateEngine)(nil)
)

type gormigrateEngineFactory struct{}

func (f *gormigrateEngineFactory) NewEngine(db *gorm.DB, logger *log.Helper) MigrationEngine {
	options := &gormigrate.Options{
		TableName:                 MigrationTableName,
		IDColumnName:              MigrationIDColumn,
		IDColumnSize:              MigrationIDSize,
		UseTransaction:            false, // Keep false to enable use of operations using CONCURRENTLY keyword
		ValidateUnknownMigrations: true,
	}

	migrationsWithLogs := MigrationsList
	if logger != nil {
		migrationsWithLogs = wrapGormigrateMigrationsWithLogger(MigrationsList, logger)
	}

	return &gormigrateEngine{
		gm: gormigrate.New(db, options, migrationsWithLogs),
	}
}

type gormigrateEngine struct {
	gm *gormigrate.Gormigrate
}

func (e *gormigrateEngine) Migrate() error {
	return e.gm.Migrate()
}

func (e *gormigrateEngine) MigrateTo(targetID string) error {
	return e.gm.MigrateTo(targetID)
}

func (e *gormigrateEngine) RollbackLast() error {
	return e.gm.RollbackLast()
}

func wrapGormigrateMigrationsWithLogger(migrations []*gormigrate.Migration, logger *log.Helper) []*gormigrate.Migration {
	wrapped := make([]*gormigrate.Migration, 0, len(migrations))
	for _, mig := range migrations {
		id := mig.ID
		migrateFn := mig.Migrate
		rollbackFn := mig.Rollback

		wrapped = append(wrapped, &gormigrate.Migration{
			ID: id,
			Migrate: func(tx *gorm.DB) error {
				logger.Infof("Running migration %s", id)
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
