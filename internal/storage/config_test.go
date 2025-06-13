package storage

import (
	"testing"

	"github.com/project-kessel/inventory-api/internal/storage/postgres"
	"github.com/project-kessel/inventory-api/internal/storage/sqlite3"

	"github.com/stretchr/testify/assert"
)

func TestConfig_Complete_WithDSN(t *testing.T) {
	cfg := &Config{
		Options: &Options{Database: "postgres"},
		DSN:     "already-set-dsn",
	}

	completed := cfg.Complete()
	assert.Equal(t, "already-set-dsn", completed.DSN)
}

func TestNewConfig_Postgres(t *testing.T) {
	opts := &Options{
		Database: "postgres",
		Postgres: &postgres.Options{},
	}
	cfg := NewConfig(opts)

	assert.Equal(t, opts, cfg.Options)
	assert.NotNil(t, cfg.Postgres)
	assert.Nil(t, cfg.SqlLite3)
}

func TestNewConfig_Sqlite3(t *testing.T) {
	opts := &Options{
		Database: "sqlite3",
		SqlLite3: &sqlite3.Options{},
	}
	cfg := NewConfig(opts)

	assert.Equal(t, opts, cfg.Options)
	assert.Nil(t, cfg.Postgres)
	assert.NotNil(t, cfg.SqlLite3)
}
