package storage

import (
	"fmt"

	"github.com/go-kratos/kratos/v2/log"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func New(c CompletedConfig, logger *log.Helper) (*gorm.DB, error) {
	var opener func(string) gorm.Dialector

	switch c.Database {
	case "postgres":
		opener = postgres.Open
	case "sqlite3":
		opener = sqlite.Open
	default:
		return nil, fmt.Errorf("unrecognized database type: %s", c.Database)
	}

	logger.Infof("Using backing storage: %s", c.Database)
	db, err := gorm.Open(opener(c.DSN), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("Error opening database: %s", err.Error())
	}

	return db, nil
}
