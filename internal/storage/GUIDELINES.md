# Infrastructure Layer Guidelines (Database Drivers)

This file covers database driver configuration under `internal/storage/`. For GORM model patterns, migrations, and repository logic, see `internal/data/GUIDELINES.md`. For cross-cutting concerns, see `internal/GUIDELINES.md`.

## Multi-Database Support
- **PostgreSQL**: Production database with full feature support
- **SQLite3**: Development/testing with limitations (no WAL outbox)

## Connection Configuration
PostgreSQL DSN building:
```go
// Supports: host, port, dbname, user, password, sslmode, sslrootcert
fmt.Fprintf(dsnBuilder, "host=%s port=%s dbname=%s...", ...)
```

## Connection Pool Strategy
- Use dual database drivers: GORM for ORM operations + pgxpool for direct PostgreSQL operations requiring connection control
- pgxpool used specifically for advanced features like advisory locks in migrations
- No explicit connection pool configuration found - relies on driver defaults
- Respect GORM's connection pooling - avoid manual connection management except for advisory locks

## Test Database Pattern
```go
// SQLite in-memory with shared cache
const SQLiteInMemoryPattern = "file:%s?mode=memory&cache=shared"
```

## Database SSL Configuration
- Support PostgreSQL SSL modes: `disable`, `allow`, `prefer`, `require`, `verify-ca`, `verify-full`
- Configure via `sslmode` and `sslrootcert` connection parameters
- Validate SSL mode against allowed values in options validation

## Certificate Management
- Load client certificates from environment variables for E2E testing
- Support CA certificate validation with custom root cert pools
- Handle certificate loading errors gracefully with informative logging
