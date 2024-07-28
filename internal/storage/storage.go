package storage

import (
	"fmt"

    "github.com/google/wire"

	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

var ProviderSet = wire.NewSet(New, NewOptions, NewConfig, NewCompleteConfig)

func New(c CompletedConfig) (*gorm.DB, error) {
	var opener func(string) gorm.Dialector

	switch c.Database {
	case "postgres":
		opener = postgres.Open
	case "sqlite3":
		opener = sqlite.Open
	default:
		return nil, fmt.Errorf("unrecognized database type: %s", c.Database)
	}

	return gorm.Open(opener(c.DSN), &gorm.Config{})
}
