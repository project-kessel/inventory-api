# Infrastructure Layer Guidelines (Kafka Consumer)

This file covers the Kafka consumer under `internal/consumer/`. For cross-cutting concerns, see `internal/GUIDELINES.md`.

## Kafka Consumer Integration

### Message Processing Pattern
- Use **structured operation handlers** with `operationConfig` for consistent create/update/delete operations
- Process messages using **Debezium CDC events** with required headers: `operation`, `txid`
- Implement **fencing tokens** via Relations API for consumer lock management during rebalancing
- Follow **batch offset commits** using `commitModulo` configuration for performance

### Consumer Configuration
- Bootstrap servers from environment or ClowdApp config via `InjectClowdAppConfig()`
- Use **SASL authentication** when `AuthConfig.Enabled` is true
- Configure **retry policies** with exponential backoff: `BackoffFactor`, `MaxBackoffSeconds`
- Set **statistics interval** for metrics collection (default: consumer stats every interval)

## Error Handling

### Message Processing Errors
- Use `ErrClosed` and `ErrMaxRetries` as sentinel errors
- Track failures with metrics: `metricscollector.Incr(i.MetricsCollector.MsgProcessFailures, operation)`
- Log context: include topic, partition, offset in error messages

### Error Classification
- **Fatal errors**: Stop consumer and propagate to error channel
- **Recoverable errors**: Log and continue processing
- **Parse errors**: Drop message and increment failure metrics

### Graceful Shutdown
- Coordinate offset commits during rebalance via `offsetMutex`
- **Retry operations** using `Retry()` method with configurable max attempts
- **Metrics collection** for failures: `MsgProcessFailures`, `ConsumerErrors`, `KafkaErrorEvents`

### Retry Strategy
Implement linear backoff with circuit breaker pattern:
```go
backoff := min(time.Duration(retryOptions.BackoffFactor*attempts*300)*time.Millisecond, 
              time.Duration(retryOptions.MaxBackoffSeconds)*time.Second)
```

## Concurrency and Thread Safety

### Offset Management Pattern
```go
type InventoryConsumer struct {
    offsetMutex        sync.Mutex
    shutdownInProgress bool
    // offsetMutex protects OffsetStorage and coordinates offset commit operations
}
```

**Rules:**
- Use dedicated mutex for each shared resource (offsetMutex for Kafka offset storage)
- Coordinate shutdown with background operations using flags (`shutdownInProgress`)
- Copy data before releasing mutex for blocking operations (commit operations)
- Restore state on operation failure while holding appropriate locks

## Performance Configuration

### Retry and Backoff Strategy
```go
// Exponential backoff with limits
backoff := min(
    time.Duration(retryOptions.BackoffFactor*attempts*300)*time.Millisecond,
    time.Duration(retryOptions.MaxBackoffSeconds)*time.Second
)
```

**Rules:**
- Use bounded linear backoff: `BackoffFactor * attempts * 300ms`
- Cap maximum backoff at configurable `MaxBackoffSeconds`
- Support unlimited retries with `OperationMaxRetries: -1` for critical operations
- Always increment attempt counters before backoff calculation

## Testing

### Consumer Testing
- **Race condition tests**: `race_condition_test.go` patterns
- **Database state**: Verify concurrent operations don't corrupt data
- **Kafka events**: Simulate Kafka messages using a fake consumer (see `internal/consumer/*_test.go` for examples)

### Test Execution
- **Race detection**: All tests run with `-race` flag in CI
- **Count verification**: `-count=1` prevents test caching issues
