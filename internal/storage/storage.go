package storage

import (
	"context"
	"fmt"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/jackc/pgx/v5/pgxpool"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func New(c CompletedConfig, logger *log.Helper) (*gorm.DB, error) {
	var opener func(string) gorm.Dialector
	var db *gorm.DB

	logger.Info("Persistence disabled: ", c.Options.DisablePersistence)

	if c.Options.DisablePersistence {
		logger.Info("Persistence disabled, skipping database connection...")
		// Return nil database connection
		return nil, nil
	}

	switch c.Options.Database {
	case "postgres":
		opener = postgres.Open
	case "sqlite3":
		opener = sqlite.Open
	default:
		return nil, fmt.Errorf("unrecognized database type: %s", c.Options.Database)
	}

	logger.Infof("Using backing storage: %s", c.Options.Database)
	db, err := gorm.Open(opener(c.DSN), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("Error opening database: %s", err.Error())
	}

	return db, nil
}
func NewPgx(c CompletedConfig, logger *log.Helper) (*pgxpool.Pool, error) {
	ctx := context.Background()
	logger.Info("Persistence disabled: ", c.Options.DisablePersistence)

	if c.Options.DisablePersistence || c.Options.Database != "postgres" {
		logger.Info("Skipping database connection for PGX...")
		return nil, nil
	}

	pool, err := pgxpool.New(ctx, c.DSN)

	if err != nil {
		return nil, fmt.Errorf("error pgx connection to DB: %v", err)
	}
	if err = pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("error pgx pinging DB: %v", err)
	}

	return pool, nil
}
