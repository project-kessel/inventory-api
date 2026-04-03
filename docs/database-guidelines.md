# Database Guidelines for Inventory API

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

### Model Constructor Pattern
All models MUST use constructor functions with validation:
```go
func NewReporterResource(...) (*ReporterResource, error) {
    rr := &ReporterResource{...}
    if err := validateReporterResource(rr); err != nil {
        return nil, err
    }
    return rr, nil
}
```

Validation uses `bizmodel.AggregateErrors()` for collecting all validation errors in one pass.

### Snapshot Serialization Pattern
Models provide serialization to/from snapshots for domain layer interaction:
```go
func (r Resource) SerializeToSnapshot() bizmodel.ResourceSnapshot
func DeserializeResourceFromSnapshot(snapshot bizmodel.ResourceSnapshot) Resource
```

## Database Schema Design

### Core Tables
- **resource**: Root entity with UUID, type, common_version, consistency_token
- **reporter_resource**: Links resources to reporters with composite natural key
- **reporter_representation**: Versioned reporter-specific data
- **common_representation**: Versioned common representation data
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

### Transaction Manager Interface
```go
type TransactionManager interface {
    HandleSerializableTransaction(operationName string, db *gorm.DB, txFunc func(tx *gorm.DB) error) error
}
```

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

### Migration ID Pattern
Use timestamp-based IDs: `20251210120000_initial_schema.go`

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

### Test Database Pattern
```go
// SQLite in-memory with shared cache
const SQLiteInMemoryPattern = "file:%s?mode=memory&cache=shared"
```

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

## Testing Patterns

### Test Database Setup
```go
func NewSQLiteTestDB(t *testing.T, cfg *gorm.Config) *gorm.DB {
    // Unique in-memory DB per test with shared cache
    dsn := fmt.Sprintf(SQLiteInMemoryPattern, testName_timestamp)
}
```

### Transaction Manager Testing
Use `fakeTransactionManager` with call tracking for unit tests.

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

### Connection Pooling
Respect GORM's connection pooling - avoid manual connection management except for advisory locks.