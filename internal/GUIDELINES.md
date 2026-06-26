# Cross-Cutting Guidelines

These guidelines apply to all code under `internal/`. Layer-specific guidelines are in the `GUIDELINES.md` file within each layer's directory.

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

## Error Logging and Context

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

### OpenTelemetry Setup
- **Meter provider** with Prometheus exporter for metrics
- **Service name**: Always use "inventory-api" for consistent identification
- **Default histogram views** for HTTP/gRPC server duration metrics
- **Resource attributes** using semantic conventions

### Metrics Collection
- **Consumer stats** with prefixed names: `consumer_stats_*`, `consumer_*`
- **Relations API calls** tracked with success/failure counters per method
- **Custom metrics** using `kessel_inventory_*` prefix for application-specific metrics
- **Label consistency**: Use structured attributes for partition, topic, operation

### Metrics Patterns
```go
// Use helper function for consistent labeling
metricscollector.Incr(counter, operation, attribute.String("extra", value))
```

## Context and Timeout Management

### Context Propagation Strategy
- Stream operations inherit context from `stream.Context()`
- Blocking operations use dedicated timeouts: `context.WithTimeout(ctx, 30*time.Second)`
- Consumer operations create fresh context for each timeout scope
- Advisory lock operations use `context.Background()` during cleanup to prevent cancellation issues

**Rules:**
- Apply 30-second timeout for PostgreSQL notification waits
- Use background context only for cleanup operations that must complete
- Always check `ctx.Err()` before interpreting timeout errors as failures

## Sensitive Data Redaction

- **Never log**: Bearer tokens, refresh tokens, API keys, passwords, JWT claims
- **Never log**: Raw identity payloads or authentication headers
- **Never log**: Resource IDs that may contain PII or sensitive identifiers
- **Redact in logs**: Use `[REDACTED]` placeholder for sensitive fields
- **Log safe data only**: Request IDs, timestamps, operation types, error codes, resource types

## Configuration Security

### Sensitive Data Management
- No plaintext secrets in configuration files
- Use environment variables for credentials and tokens
- Support insecure modes only for development/testing
- Validate configuration options at startup

## Development and Testing Security

### Test Data and Mocks
- Use example PSKs in test configurations only
- Implement security-aware test utilities in `testutil/`
- Mock authenticators and authorizers for unit tests
- Separate test credentials from production paths

### Environment-Specific Settings
- Support TLS environment variables for E2E testing
- Graceful fallback to insecure mode when certificates unavailable
- Clear separation between development and production security settings

## Configuration Management

### Universal Configuration Pattern
Every component follows the same configuration lifecycle:
```go
// 1. Options struct with mapstructure tags
type Options struct {
    Field1 string `mapstructure:"field1"`
    Field2 bool   `mapstructure:"field2"`
}

// 2. Constructor with sensible defaults
func NewOptions() *Options {
    return &Options{Field1: "default", Field2: true}
}

// 3. Command-line flags via pflag
func (o *Options) AddFlags(fs *pflag.FlagSet, prefix string) {
    fs.StringVar(&o.Field1, prefix+"field1", o.Field1, "description")
}

// 4. Configuration completion and normalization
func (o *Options) Complete() []error {
    // Apply defaults, normalize values
    return nil
}

// 5. Configuration validation
func (o *Options) Validate() []error {
    // Return all validation errors as slice
    return errs
}
```

### ClowdApp Integration
- **Automatic discovery** of dependent services via `InjectClowdAppConfig()`
- **Database configuration** from ClowdApp database section
- **Kafka brokers** extracted from ClowdApp kafka configuration
- **RDS CA certificate** handling for TLS connections

### Environment-Based Config
- **Structured options pattern**: Each component has `*Options` and `CompletedConfig`
- **Validation on completion**: Return errors array from `Complete()` methods
- **Debug logging**: Use `LogConfigurationInfo()` to output non-sensitive config details

## Testing Philosophy (Summary)

**Prefer test-driven development (TDD).** No "method-verifying" mocks. Preference order: real instances > fakes > stubs/dummies. See `test/GUIDELINES.md` for the full testing philosophy and patterns.
