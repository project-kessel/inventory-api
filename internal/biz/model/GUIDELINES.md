# Domain Layer Guidelines

This file covers the domain model under `internal/biz/model/`. For application services, see `internal/biz/usecase/GUIDELINES.md`. For cross-cutting concerns, see `internal/GUIDELINES.md`. For the full domain model conventions (tiny types, value objects, constructors), see `AGENTS.md` at the repo root.

## Error Types

### Sentinel Errors in `model` Package
- **Validation errors**: `ErrEmpty`, `ErrTooLong`, `ErrTooSmall`, `ErrInvalidURL`, `ErrInvalidUUID`
- **Domain errors**: `ErrResourceNotFound`, `ErrResourceAlreadyExists`, `ErrInventoryIdMismatch`, `ErrVersionConflict`
- **Service errors**: `ErrDatabaseError`, `ErrInvalidData`

### Validation Aggregation
Use `model.AggregateErrors()` for validation failures - automatically converts to `ValidationError` type with field context.

## Model Constructor Pattern
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

## Snapshot Serialization Pattern
Models provide serialization to/from snapshots for domain layer interaction:
```go
func (r Resource) SerializeToSnapshot() bizmodel.ResourceSnapshot
func DeserializeResourceFromSnapshot(snapshot bizmodel.ResourceSnapshot) Resource
```

## Domain Representation of API Types

### Resource Reference Structure
```proto
message ResourceReference {
  string resource_type = 1;     // Validated: ^[A-Za-z0-9_]+$
  string resource_id = 2;       // Min length: 1
  optional ReporterReference reporter = 3;  // For multi-reporter resources
}
```

### Consistency Model
Three consistency levels available:
1. `minimize_latency` - fastest, eventually consistent
2. `at_least_as_fresh` - token-based consistency
3. `at_least_as_acknowledged` - strong consistency

Usage pattern:
- Default: `minimize_latency` for read operations
- Use `at_least_as_acknowledged` for write-then-read scenarios
- Use `at_least_as_fresh` for coordinated reads across operations

### Write Visibility Patterns
- `MINIMIZE_LATENCY` (default): Async writes, higher throughput
- `IMMEDIATE`: Synchronous writes, read-your-writes consistency
- Use `IMMEDIATE` when subsequent Check operations depend on the write

## Testing

### Assertion Style
- **Assert full objects, not individual attributes**: When verifying a result, compare against the complete expected value using `assert.Equal(t, expectedObject, actualObject)`. Do not decompose objects and assert on individual fields -- this obscures intent, is fragile to refactoring, and misses unexpected field values. For example, use `assert.Equal(t, model.NewWorkspaceRelationsTuple("ws-1", key), tuple)` rather than separately asserting on `tuple.Object().ResourceId()`, `tuple.Subject().Resource().ResourceId()`, etc.

### Error Testing Patterns
- **gRPC codes**: Use `codes.InvalidArgument`, `codes.NotFound`, etc.
- **Status verification**: Check both gRPC status and HTTP status codes
- **Error message validation**: Assert on specific error text for UX

### Resource Lifecycle Testing

#### Versioning Patterns
Test these state transitions extensively:
- **Generation**: Increments when resource revived from tombstone
- **RepresentationVersion**: Increments on each data change  
- **Tombstone**: Tracks deletion state
- **Idempotency**: Same transaction ID = no version changes
