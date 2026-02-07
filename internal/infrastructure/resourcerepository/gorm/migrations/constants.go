package migrations

const (
	// Migration table metadata
	MigrationTableName = "migrations"
	MigrationIDColumn  = "id"
	MigrationIDSize    = 255

	// SQLite
	SQLitePragmaForeignKeysOn = "PRAGMA foreign_keys = ON"
)
