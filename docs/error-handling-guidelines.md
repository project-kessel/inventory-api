# Inventory API Error Handling Guidelines

## Error Type Hierarchy

### Sentinel Errors in `model` Package
- **Validation errors**: `ErrEmpty`, `ErrTooLong`, `ErrTooSmall`, `ErrInvalidURL`, `ErrInvalidUUID`
- **Domain errors**: `ErrResourceNotFound`, `ErrResourceAlreadyExists`, `ErrInventoryIdMismatch`, `ErrVersionConflict`
- **Service errors**: `ErrDatabaseError`, `ErrInvalidData`

### Application-Layer Errors in `usecase/resources`
- **Authorization errors**: `ErrMetaAuthzContextMissing`, `ErrSelfSubjectMissing`, `ErrMetaAuthorizerUnavailable`, `ErrMetaAuthorizationDenied`
- Re-export domain errors: `ErrResourceNotFound = model.ErrResourceNotFound`

## Error Mapping Strategy

### gRPC Status Code Mapping
Use `middleware.MapError()` for consistent error-to-status mapping:
- Validation errors → `InvalidArgument`
- Authorization errors → `Unauthenticated` or `PermissionDenied`
- Domain errors → Appropriate business codes (`NotFound`, `AlreadyExists`, `FailedPrecondition`)
- Database errors → `Internal`
- Context errors → `Canceled` or `DeadlineExceeded`

### Preserve Existing Status Codes
`MapError` passes through errors that already have gRPC status codes (except `Unknown`)

## Consumer Error Handling

### Message Processing
- Use `ErrClosed` and `ErrMaxRetries` as sentinel errors
- Track failures with metrics: `metricscollector.Incr(i.MetricsCollector.MsgProcessFailures, operation)`
- Log context: include topic, partition, offset in error messages

### Error Classification
- **Fatal errors**: Stop consumer and propagate to error channel
- **Recoverable errors**: Log and continue processing
- **Parse errors**: Drop message and increment failure metrics

### Retry Strategy
Implement exponential backoff with circuit breaker pattern:
```go
backoff := min(time.Duration(retryOptions.BackoffFactor*attempts*300)*time.Millisecond, 
              time.Duration(retryOptions.MaxBackoffSeconds)*time.Second)
```

## Transaction Error Handling

### Serialization Failures
- Auto-retry on PostgreSQL error code `40001` and SQLite `ErrError`
- Track metrics: `SerializationFailures` and `SerializationExhaustions`
- Maximum retry attempts configurable via `maxSerializationRetries`

### Outbox Pattern
- Handle failures in both WAL and table-based modes
- Wrap marshaling/unmarshaling errors: `fmt.Errorf("failed to marshal WAL message: %w", err)`

## Error Aggregation

### Custom Aggregation Types
Use `internal/errors.Aggregate` for collecting multiple errors:
```go
type Aggregate struct {
    Errors []error
}
```

### Validation Aggregation
Use `model.AggregateErrors()` for validation failures - automatically converts to `ValidationError` type with field context.

## Context and Logging

### Error Logging Strategy
- Application errors: Log before mapping to gRPC status
- Include operation context in error messages
- Use structured logging with consistent field names

### Context Propagation
- Map `context.Canceled` and `context.DeadlineExceeded` to appropriate gRPC codes
- Use timeouts for external operations: `context.WithTimeout(ctx, listenTimeout)`

## Testing Error Conditions

### Test Utilities
Use `internal/errors` test helpers:
- `AssertIs(t, got, want)` for sentinel error checks
- `AssertErrorContains(t, err, substring)` for wrapped error validation

### Error Mapping Tests
Test wrapped errors with `errors.Is()` compatibility:
```go
err := fmt.Errorf("operation failed: %w", model.ErrResourceNotFound)
// Should map correctly to NotFound status
```

## Retry and Circuit Breaker Patterns

### Consumer Retry Logic
- Separate retry counts for consumer lifecycle vs individual operations
- Use `operation-max-retries` for Relations API calls
- Implement backoff with configurable factors

### Circuit Breaker Usage
- Use `gobreaker` for protecting against cascading failures
- Configure for read-after-write notification waiting
- Log state changes for monitoring

## Metrics and Observability

### Error Metrics
Track specific error categories:
- `MsgProcessFailures` with operation labels
- `ConsumerErrors` with error type labels  
- `SerializationFailures` and `SerializationExhaustions`

### Error Context
Include relevant identifiers in errors:
- Transaction IDs for duplicate detection
- Resource keys for resource operations
- Client IDs for authorization context

## Repository Error Patterns

### Database Error Wrapping
- Detect `gorm.ErrDuplicatedKey` and map to business errors
- Wrap repository errors: `fmt.Errorf("failed to save resource: %w", err)`
- Return sentinel errors for business logic violations

### Consistency Token Handling
- Skip updates when tokens are empty rather than failing
- Log debug messages for skipped operations
- Separate concerns: token presence vs operation success