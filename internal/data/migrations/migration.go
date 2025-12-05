package migrations

import (
	"fmt"
	"time"

	"github.com/go-gormigrate/gormigrate/v2"

	schema "github.com/project-kessel/inventory-api/internal/data/migrations/schema"
)

// Ordered list of schema migrations.
// IDs must be timestamp-based and strictly increasing.
var MigrationsList = []*gormigrate.Migration{
	schema.InitialSchema(),
}

func init() {
	if err := validateMigrationsList(MigrationsList); err != nil {
		panic(err)
	}
}

func validateMigrationsList(migrations []*gormigrate.Migration) error {
	const migrationIDLayout = "20060102150405"

	if len(migrations) == 0 {
		return nil
	}

	var prev time.Time
	for i, m := range migrations {
		if m == nil {
			return fmt.Errorf("migration[%d] is nil", i)
		}
		if m.ID == "" {
			return fmt.Errorf("migration[%d] has empty ID", i)
		}

		current, err := time.Parse(migrationIDLayout, m.ID)
		if err != nil {
			return fmt.Errorf("migration[%d] ID %q is not a valid timestamp (expected format yyyyMMddHHmmss): %w", i, m.ID, err)
		}

		if i > 0 && !current.After(prev) {
			return fmt.Errorf("migration[%d] ID %q must be strictly later than previous ID %q", i, m.ID, migrations[i-1].ID)
		}

		prev = current
	}

	return nil
}
