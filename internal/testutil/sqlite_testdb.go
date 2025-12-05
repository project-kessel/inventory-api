package testutil

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

const SQLiteInMemoryPattern = "file:%s?mode=memory&cache=shared"

func NewSQLiteTestDB(t *testing.T, cfg *gorm.Config) *gorm.DB {
	t.Helper()

	if cfg == nil {
		cfg = &gorm.Config{}
	}

	// Use the test name plus a timestamp to generate a unique in-memory
	// database per test
	safeName := strings.ReplaceAll(t.Name(), "/", "_")
	dsn := fmt.Sprintf(SQLiteInMemoryPattern, fmt.Sprintf("%s_%d", safeName, time.Now().UnixNano()))

	db, err := gorm.Open(sqlite.Open(dsn), cfg)
	require.NoError(t, err)

	return db
}
