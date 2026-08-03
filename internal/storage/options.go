package storage

import (
	"errors"

	"github.com/spf13/pflag"

	"github.com/project-kessel/inventory-api/internal/storage/postgres"
	"github.com/project-kessel/inventory-api/internal/storage/sqlite3"
)

type Options struct {
	Postgres                *postgres.Options `mapstructure:"postgres"`
	SqlLite3                *sqlite3.Options  `mapstructure:"sqlite3"`
	Database                string            `mapstructure:"database"`
	MaxSerializationRetries int               `mapstructure:"max-serialization-retries"`
	OutboxMode              string            `mapstructure:"outbox-mode"`
}

const (
	Postgres = "postgres"
	Sqlite3  = "sqlite3"

	OutboxModeWAL = "wal"
	// OutboxModeNone disables outbox publishing. Use with SQLite or any deployment
	// that does not have a Debezium/Kafka consumer pipeline. Resource writes succeed
	// but no events are emitted downstream.
	OutboxModeNone = "none"
)

func NewOptions() *Options {
	return &Options{
		Postgres:                postgres.NewOptions(),
		SqlLite3:                sqlite3.NewOptions(),
		Database:                "sqlite3",
		MaxSerializationRetries: 10,
		OutboxMode:              OutboxModeNone,
	}
}

func (o *Options) AddFlags(fs *pflag.FlagSet, prefix string) {
	if prefix != "" {
		prefix = prefix + "."
	}

	fs.StringVar(&o.Database, prefix+"database", o.Database, "The database type to use.  Either sqlite3 or postgres.")
	fs.IntVar(&o.MaxSerializationRetries, prefix+"max-serialization-retries", o.MaxSerializationRetries, "Maximum number of retries for serialized transactions")
	fs.StringVar(&o.OutboxMode, prefix+"outbox-mode", o.OutboxMode, "'wal' (pg_logical_emit_message) or 'none' for sqlite/standalone use")

	o.Postgres.AddFlags(fs, prefix+"postgres")
	o.SqlLite3.AddFlags(fs, prefix+"sqlite3")
}

func (o *Options) Complete() []error {
	return nil
}

func (o *Options) Validate() []error {
	var errs []error
	if o.Database != "postgres" && o.Database != "sqlite3" {
		errs = append(errs, errors.New("database must be either postgres or sqlite3"))
	}

	switch o.Database {
	case "postgres":
		errs = append(errs, o.Postgres.Validate()...)
	case "sqlite3":
		errs = append(errs, o.SqlLite3.Validate()...)
	}

	if o.OutboxMode != OutboxModeNone && o.OutboxMode != OutboxModeWAL {
		errs = append(errs, errors.New("outbox-mode must be either 'none' or 'wal'"))
	}

	if o.OutboxMode == OutboxModeWAL && o.Database != Postgres {
		errs = append(errs, errors.New("outbox-mode 'wal' is only supported with postgres"))
	}

	return errs
}
