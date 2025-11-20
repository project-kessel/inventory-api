package migrations

const (
	// Migration table metadata
	MigrationTableName = "migrations"
	MigrationIDColumn  = "id"
	MigrationIDSize    = 255

	// Lock types
	LockTypeMigrations = "migrations"

	// SQLite
	SQLitePragmaForeignKeysOn = "PRAGMA foreign_keys = ON"
)
