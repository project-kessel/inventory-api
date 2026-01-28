package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/jackc/pgx/v5/pgxpool"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// NewStorage creates a new gorm.DB instance based on the storage options.
func NewStorage(opts *StorageOptions, logger *log.Helper) (*gorm.DB, error) {
	if errs := opts.Validate(); len(errs) > 0 {
		return nil, fmt.Errorf("storage validation failed: %v", errs)
	}

	var opener func(string) gorm.Dialector
	dsn := buildDSN(opts)

	switch opts.Database {
	case DatabasePostgres:
		opener = postgres.Open
	case DatabaseSqlite3:
		opener = sqlite.Open
	default:
		return nil, fmt.Errorf("unrecognized database type: %s", opts.Database)
	}

	logger.Infof("Using backing storage: %s", opts.Database)
	db, err := gorm.Open(opener(dsn), &gorm.Config{TranslateError: true})
	if err != nil {
		return nil, fmt.Errorf("error opening database: %s", err.Error())
	}

	return db, nil
}

// NewPgxPool creates a new pgxpool.Pool instance for PostgreSQL.
// Returns nil if the database type is not postgres.
func NewPgxPool(ctx context.Context, opts *StorageOptions, logger *log.Helper) (*pgxpool.Pool, error) {
	if opts.Database != DatabasePostgres {
		logger.Info("Skipping database connection for PGX...")
		return nil, nil
	}

	dsn := buildDSN(opts)
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("error pgx connection to DB: %v", err)
	}
	if err = pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("error pgx pinging DB: %v", err)
	}

	return pool, nil
}

// buildDSN builds a connection string from the storage options.
func buildDSN(opts *StorageOptions) string {
	switch opts.Database {
	case DatabasePostgres:
		return buildPostgresDSN(opts.Postgres)
	case DatabaseSqlite3:
		return opts.SqlLite3.DSN
	default:
		return ""
	}
}

// buildPostgresDSN builds a PostgreSQL connection string from options.
func buildPostgresDSN(opts *PostgresOptions) string {
	dsnBuilder := new(strings.Builder)

	if opts.Host != "" {
		fmt.Fprintf(dsnBuilder, "host=%s ", opts.Host)
	}

	if opts.Port != "" {
		fmt.Fprintf(dsnBuilder, "port=%s ", opts.Port)
	}

	if opts.DbName != "" {
		fmt.Fprintf(dsnBuilder, "dbname=%s ", opts.DbName)
	}

	if opts.User != "" {
		fmt.Fprintf(dsnBuilder, "user=%s ", opts.User)
	}

	if opts.Password != "" {
		fmt.Fprintf(dsnBuilder, "password=%s ", opts.Password)
	}

	if opts.SSLMode != "" {
		fmt.Fprintf(dsnBuilder, "sslmode=%s ", opts.SSLMode)
	}

	if opts.SSLRootCert != "" {
		fmt.Fprintf(dsnBuilder, "sslrootcert=%s ", opts.SSLRootCert)
	}

	return strings.TrimSpace(dsnBuilder.String())
}

// StorageResult contains the results of storage initialization.
type StorageResult struct {
	DB      *gorm.DB
	PgxPool *pgxpool.Pool
}

// NewStorageAll creates both gorm.DB and pgxpool.Pool instances.
func NewStorageAll(ctx context.Context, opts *StorageOptions, logger *log.Helper) (*StorageResult, error) {
	db, err := NewStorage(opts, logger)
	if err != nil {
		return nil, err
	}

	pool, err := NewPgxPool(ctx, opts, logger)
	if err != nil {
		return nil, err
	}

	return &StorageResult{
		DB:      db,
		PgxPool: pool,
	}, nil
}
