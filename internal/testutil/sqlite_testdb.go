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

// Use a named in-memory DB with shared cache so GORM's connection pool
// sees a single logical database. ":memory:" would create a separate
// DB per connection and break tests.
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
