# Infrastructure Layer Guidelines (Data/Persistence)

This file covers the data layer under `internal/data/`. For database driver configuration, see `internal/storage/GUIDELINES.md`. For cross-cutting concerns, see `internal/GUIDELINES.md`.

## Model Structure and Conventions

### Field Size Constants
Use predefined field size constants from `internal/data/model/common.go`:
- `MaxFieldSize128/256/512/1024` for string fields  
- Specific constants like `MaxLocalResourceIDLength`, `MaxReporterTypeLength`
- Consistent min value constants: `MinVersionValue`, `MinGenerationValue`, `MinCommonVersion`

### GORM Model Patterns

**Primary Keys:**
- Always use `uuid.UUID` with `gorm:"type:uuid;primaryKey"`
- No auto-incrementing IDs - UUIDs only

**String Field Sizing:**
```go
ReporterType string `gorm:"size:128;not null"`
APIHref      string `gorm:"size:512;not null"`
```

**Composite Keys and Indexes:**
```go
// Multi-column unique index with priorities
LocalResourceID string `gorm:"size:256;index:reporter_resource_key_idx,unique;index:reporter_resource_search_idx,priority:1;not null"`
```

**JSONB for Flexible Data:**
```go
Data internal.JsonObject `gorm:"type:jsonb"`
```

**Check Constraints:**
```go
Version uint `gorm:"type:bigint;check:version >= 0"`
```

## Database Schema Design

### Core Tables
- **resource**: Root entity with UUID, type, common_version, consistency_token
- **reporter_resources**: Links resources to reporters with composite natural key
- **reporter_representations**: Versioned reporter-specific data
- **common_representations**: Versioned common representation data
- **outbox_events**: Event sourcing for external integrations

### Composite Key Design
ReporterResourceKey combines 4 fields to uniquely identify resources:
- LocalResourceID, ReporterType, ResourceType, ReporterInstanceID
- Creates both unique and search indexes with explicit priorities

### Foreign Key Relationships
```go
ReporterResource ReporterResource `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;foreignKey:ReporterResourceID;references:ID"`
```

## Transaction Management

### Serializable Transaction Pattern
Always use the transaction manager for consistency:
```go
err = tm.HandleSerializableTransaction("operation_name", db, func(tx *gorm.DB) error {
    // Database operations here
    return nil
})
```

**Key Features:**
- Automatic retry on serialization failures (PostgreSQL error 40001, SQLite ErrError)
- Configurable max retry attempts (`MaxSerializationRetries`)
- Metrics collection for failures and exhaustions
- Isolation level: `sql.LevelSerializable`

**See also:** [Serializable Isolation Level Guide](../dev-guides/serializable-isolation-level.md) for best practices and configuration details.

**Rules:**
- Always use `HandleSerializableTransaction` for resource CRUD operations
- Keep transactions lean - avoid complex multi-table operations in single transaction
- All reads MUST be within the same transaction as writes for conflict detection
- Never use explicit locks (`SELECT FOR UPDATE`) - serializable isolation provides protection
- Configure `max-serialization-retries` (default: 10) based on contention analysis

### Transaction Manager Interface
```go
type TransactionManager interface {
    HandleSerializableTransaction(operationName string, db *gorm.DB, txFunc func(tx *gorm.DB) error) error
}
```

### Transaction Error Handling

#### Serialization Failures
- Auto-retry on PostgreSQL error code `40001` and SQLite `ErrError`
- Track metrics: `SerializationFailures` and `SerializationExhaustions`
- Maximum retry attempts configurable via `maxSerializationRetries`

#### Outbox Pattern
- Handle failures in both WAL and table-based modes
- Wrap marshaling/unmarshaling errors: `fmt.Errorf("failed to marshal WAL message: %w", err)`

## Outbox Pattern Implementation

### Dual Mode Support
1. **Table Mode** (`outbox-mode=table`): Traditional outbox_events table
2. **WAL Mode** (`outbox-mode=wal`): PostgreSQL logical decoding with `pg_logical_emit_message`

### WAL Implementation
```go
// Message published within transaction boundary
tx.Exec("SELECT pg_logical_emit_message(true, ?, ?)", prefix, content)
```

WAL mode only works with PostgreSQL and provides better performance by avoiding table-based outbox.

### Publisher Function Pattern
```go
type OutboxPublisher func(tx *gorm.DB, event *model_legacy.OutboxEvent) error
```

### Dual Publishing Strategy
- **WAL logical decoding** mode for high performance (`storage.OutboxModeWAL`)
- **Table-based outbox** as fallback for compatibility
- **Message format consistency** between both approaches using `walOutboxMessage`

### Transaction Coordination
- **Outbox events** created within same transaction as domain changes
- **UUID generation** using V7 for time-ordered IDs
- **Payload preservation** using `internal.JsonObject` for flexible content

## Migration System

### Gormigrate Integration
- Uses `go-gormigrate/gormigrate/v2` for migration management
- Migration table: configurable name with ID and size constraints
- `UseTransaction: false` to enable `CONCURRENTLY` DDL operations

### Advisory Lock Pattern (PostgreSQL)
```go
func WithAdvisoryLock(ctx context.Context, db *gorm.DB, fn func(*gorm.DB) error) error
```

**Critical Design:**
- Dedicated connection holds advisory lock (`pg_advisory_lock`)
- GORM operations bound to locked connection via `connWrapper`
- Prevents concurrent migrations across multiple instances
- Enables non-transactional DDL like `CREATE INDEX CONCURRENTLY`

**Rules:**
- Use dedicated connections for advisory locks to ensure lock ownership
- Bind GORM sessions to specific connections when lock ownership matters
- Always defer connection cleanup even if unlock fails
- Use background context for best-effort unlock operations

### Migration ID Pattern
Use timestamp-based IDs: `20251210120000_initial_schema.go`

### Migration Management
**See:** [Database Migrations Guide](../dev-guides/migrations.md) for migration creation, execution, and best practices including:
- Creating new migrations with proper naming and timestamps
- Using advisory locks for safe concurrent migration execution
- Rollback procedures and migration validation

## Database Configuration

### Multi-Database Support
- **PostgreSQL**: Production database with full feature support
- **SQLite3**: Development/testing with limitations (no WAL outbox)

### Connection Configuration
PostgreSQL DSN building:
```go
// Supports: host, port, dbname, user, password, sslmode, sslrootcert
fmt.Fprintf(dsnBuilder, "host=%s port=%s dbname=%s...", ...)
```

For driver-level connection configuration, see `internal/storage/GUIDELINES.md`.

### Database SSL Configuration
- Support PostgreSQL SSL modes: `disable`, `allow`, `prefer`, `require`, `verify-ca`, `verify-full`
- Configure via `sslmode` and `sslrootcert` connection parameters
- Validate SSL mode against allowed values in options validation

## Database Connection Management

### Connection Pool Strategy
- Use dual database drivers: GORM for ORM operations + pgxpool for direct PostgreSQL operations requiring connection control
- pgxpool used specifically for advanced features like advisory locks in migrations
- No explicit connection pool configuration found - relies on driver defaults

### Connection Pooling
Respect GORM's connection pooling - avoid manual connection management except for advisory locks.

## Query and Repository Patterns

### Repository Structure Queries
Use structured result types for complex joins:
```go
type FindResourceByKeysResult struct {
    ReporterResourceID uuid.UUID `gorm:"column:reporter_resource_id"`
    // ... other fields with explicit column mapping
}
```

### Column Name Constants
Define column names as constants in `common.go`:
```go
const (
    ColumnResourceID     = "id"
    ColumnVersion        = "version"
    ColumnReporterType   = "reporter_type"
)
```

### Index Naming Convention
- Primary composite: `reporter_resource_key_idx`
- Search optimized: `reporter_resource_search_idx`
- Foreign key: `reporter_resource_resource_id_idx`

### Repository Error Patterns

#### Database Error Wrapping
- Detect `gorm.ErrDuplicatedKey` and map to business errors
- Wrap repository errors: `fmt.Errorf("failed to save resource: %w", err)`
- Return sentinel errors for business logic violations

#### Consistency Token Handling
- Skip updates when tokens are empty rather than failing
- Log debug messages for skipped operations
- Separate concerns: token presence vs operation success

## Relations API Integration

### Client Authentication
- Use **OIDC token client** when `EnableOIDCAuth` is true
- Apply **Bearer token** auth via `WithBearerToken()` or `WithInsecureBearerToken()`
  - ⚠️ **Security Warning**: `WithInsecureBearerToken()` bypasses TLS verification and should never be used in production
- Handle **authentication failures** with proper error metrics

### Tuple Operations
- **Convert internal models** to protobuf using `convertTuplesToRelationships()`
- Use **upsert semantics** for CreateTuples to handle existing relationships
- Implement **consistency tokens** for read-after-write guarantees via `UpdateConsistencyToken()`
- Apply **fencing checks** for all tuple operations to prevent race conditions

### Kessel Relations-API Client
- Use gRPC client with proper bearer token authentication
- Implement token caching with 5-minute expiration via `tokenClient`
- Default to `minimize_latency` consistency for non-existent resources
- Use `at_least_as_fresh` consistency when resource exists (with token)

## Schema Validation

### JSON Schema Integration
- **gojsonschema library** for validation with structured error reporting
- **ValidationSchema interface** for consistent validation across different data types
- **Error aggregation**: Combine multiple validation errors into single error message

### Resource Schema Management
- **Schema repository pattern** with in-memory caching
- **Resource type definitions** loaded from tarball during deployment
- **Representation validation** against reporter-specific schemas

## Service Discovery

### ClowdApp Dependencies
- **Optional dependencies** declared in deployment template for Relations API
- **Hostname/port resolution** via ClowdApp endpoint configuration
- **Automatic URL construction** for dependent service connections

### Configuration Injection
```go
// Standard pattern for ClowdApp integration
func (o *OptionsConfig) InjectClowdAppConfig(appconfig *clowder.AppConfig) error
```

## Performance Considerations

### Database Type Specifications
```go
DBTypeText   = "text"     // For flexible text
DBTypeBigInt = "bigint"   // For versions/counts  
DBTypeJSONB  = "jsonb"    // For structured data
```

### Constraint Usage
- Check constraints for value ranges: `check:version >= 0`
- Unique constraints with conditional WHERE clauses
- Size limits prevent excessive memory usage

## Testing

### Test Database Setup
```go
func NewSQLiteTestDB(t *testing.T, cfg *gorm.Config) *gorm.DB {
    // Unique in-memory DB per test with shared cache
    dsn := fmt.Sprintf(SQLiteInMemoryPattern, testName_timestamp)
}
```

### Test Database Pattern
```go
// SQLite in-memory with shared cache
const SQLiteInMemoryPattern = "file:%s?mode=memory&cache=shared"
```

### Transaction Manager Testing
Use `fakeTransactionManager` with call tracking for unit tests.

### Test Double Strategy

**Prefer real instances and fakes over mocks.** The codebase provides a rich set of in-memory fakes that serve as both test infrastructure and a feature of the application itself:

| Implementation | Location | Purpose |
|----------------|----------|---------|
| `NewFakeResourceRepository()` | `internal/data/fake_resource_repository.go` | Full in-memory resource storage with mutex and transaction ID tracking |
| `NewFakeTransactionManager()` | `internal/data/fake_transaction_manager.go` | No-op transaction handling for tests that don't need real DB transactions |
| `NewFakeMetricsCollector()` | `internal/metricscollector/fake_metricscollector.go` | No-op OTEL counters for tests that need a `MetricsCollector` shape |
| `NewSimpleRelationsRepository()` | `internal/data/relations_simple.go` | In-process Relations API behavior (tuples, checks, snapshots) |

Note: `NewInMemorySchemaRepository()` (`internal/data/schema_inmemory.go`) is the **production** schema store, not a fake. It is used directly in tests as a real instance — a good example of preferring the real thing when no external I/O is involved.

These fakes are not throw-away test code — they are **reusable, evolvable implementations** that encode domain knowledge and can be used to run the service with zero external dependencies.

### Contract Testing

When implementing a fake, define contract tests that run against both the real and fake implementations. This ensures the fake faithfully implements the interface contract.

```go
// Contract testing - test both real and fake implementations
implementations := []struct {
    name string
    repo func() bizmodel.ResourceRepository
}{
    {"Real Repository", func() { return NewResourceRepository(db, tm, publisher) }},
    {"Fake Repository", func() { return NewFakeResourceRepository() }},
}

for _, impl := range implementations {
    t.Run(impl.name, func(t *testing.T) {
        testRepositoryContract(t, impl.repo())
    })
}
```

See `internal/data/resource_repository_test.go` for the canonical example of this pattern.

### Database Testing
- **In-memory SQLite**: Use `testutil.NewSQLiteTestDB(t, cfg)` for tests that need real SQL behavior
- **Unique databases**: Each test gets isolated DB via test name + timestamp
- **Migrations**: Run `data.Migrate(db, nil)` after DB creation for consistent schema
- **Pattern**: `db := setupInMemoryDB(t)` helper in most test files
- **When to use SQLite vs fake**: Use the fake repo for business logic tests. Use SQLite when the test specifically needs SQL/GORM behavior (e.g., testing query construction, migration logic, or constraint handling).

### Schema Testing
- **In-memory repository**: `data.NewInMemorySchemaRepository()` for tests
- **Validation schemas**: JSON Schema strings embedded in test setup
- **Resource/reporter combinations**: Pre-configure valid type mappings
