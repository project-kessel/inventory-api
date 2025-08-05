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
}

const (
	Postgres = "postgres"
	Sqlite3  = "sqlite3"
)

func NewOptions() *Options {
	return &Options{
		Postgres:                postgres.NewOptions(),
		SqlLite3:                sqlite3.NewOptions(),
		Database:                "sqlite3",
		MaxSerializationRetries: 10,
	}
}

func (o *Options) AddFlags(fs *pflag.FlagSet, prefix string) {
	if prefix != "" {
		prefix = prefix + "."
	}

	fs.StringVar(&o.Database, prefix+"database", o.Database, "The database type to use.  Either sqlite3 or postgres.")
	fs.IntVar(&o.MaxSerializationRetries, prefix+"max-serialization-retries", o.MaxSerializationRetries, "Maximum number of retries for serialized transactions")

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

	return errs
}
