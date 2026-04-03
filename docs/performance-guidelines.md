# Performance Guidelines for Inventory API

## Database Connection Management

### Connection Pool Strategy
- Use dual database drivers: GORM for ORM operations + pgxpool for direct PostgreSQL operations requiring connection control
- pgxpool used specifically for advanced features like advisory locks in migrations
- No explicit connection pool configuration found - relies on driver defaults

### Transaction Patterns
```go
// Always use serializable isolation with automatic retry
func (tm *gormTransactionManager) HandleSerializableTransaction(operationName string, db *gorm.DB, txFunc func(tx *gorm.DB) error) error {
    for i := 0; i < tm.maxSerializationRetries; i++ {
        tx := db.Begin(&sql.TxOptions{Isolation: sql.LevelSerializable})
        // ... retry logic for PostgreSQL error code 40001
    }
}
```

**Rules:**
- Always use `HandleSerializableTransaction` for resource CRUD operations
- Keep transactions lean - avoid complex multi-table operations in single transaction
- All reads MUST be within the same transaction as writes for conflict detection
- Never use explicit locks (`SELECT FOR UPDATE`) - serializable isolation provides protection
- Configure `max-serialization-retries` (default: 10) based on contention analysis

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

### Channel-Based Communication
```go
// PubSub pattern with buffered channels
func (l *ListenManager) Subscribe(txId string) Subscription {
    sub := &subscription{
        listenChan: make(chan []byte, 2), // Small buffer to prevent blocking
    }
}
```

**Rules:**
- Use small channel buffers (size 2) for notification channels to prevent memory buildup
- Drop notifications on full buffers with logging rather than blocking
- Implement timeout-based operations with `context.WithTimeout(ctx, 30*time.Second)`
- Use `sync.Once` for cleanup operations that should only run once

## Streaming and Large Dataset Handling

### gRPC Streaming Implementation
```go
func (s *InventoryService) StreamedListObjects(req *pb.StreamedListObjectsRequest, stream pb.KesselInventoryService_StreamedListObjectsServer) error {
    for {
        resp, err := clientStream.Recv()
        if err == io.EOF { return nil }
        if err := stream.Send(ToLookupResourceResponse(resp)); err != nil { return err }
    }
}
```

**Rules:**
- Use direct stream forwarding without buffering for large datasets
- Check for `io.EOF` to detect normal stream completion
- Always validate stream context cancellation
- Convert between internal and external protobuf types at streaming boundaries

## Context and Timeout Management

### Context Propagation Strategy
- Stream operations inherit context from `stream.Context()`
- Blocking operations use dedicated timeouts: `context.WithTimeout(ctx, 30*time.Second)`
- Consumer operations create fresh context for each timeout scope
- Advisory lock operations use `context.Background()` during cleanup to prevent cancellation issues

**Rules:**
- Use stream context for gRPC streaming operations
- Apply 30-second timeout for PostgreSQL notification waits
- Use background context only for cleanup operations that must complete
- Always check `ctx.Err()` before interpreting timeout errors as failures

## Resource Management and Cleanup

### Advisory Lock Pattern for Migrations
```go
func WithAdvisoryLock(ctx context.Context, db *gorm.DB, fn func(*gorm.DB) error) error {
    conn, err := sqlDB.Conn(ctx) // Dedicated connection
    defer conn.Close()
    
    // Bind GORM operations to locked connection
    connDB.Statement.ConnPool = &connWrapper{conn: conn}
    return fn(connDB)
}
```

**Rules:**
- Use dedicated connections for advisory locks to ensure lock ownership
- Bind GORM sessions to specific connections when lock ownership matters
- Always defer connection cleanup even if unlock fails
- Use background context for best-effort unlock operations

## Metrics and Performance Monitoring

### Custom Stream Metrics
```go
// Separate metrics for stream connections vs. individual messages
const (
    StreamCounterName = "grpc_server_streams_total"           // Per-stream
    StreamMessageCounterName = "grpc_server_stream_messages_total" // Per-message
)
```

**Rules:**
- Track stream connections separately from message counts
- Use `sync.Once` to record first-response latency only once per stream
- Implement custom interceptors for accurate streaming metrics (Kratos v2.9.X has inflated counts)
- Record both sent/received direction attributes for message metrics

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
- Use bounded exponential backoff: `BackoffFactor * attempts * 300ms`
- Cap maximum backoff at configurable `MaxBackoffSeconds`
- Support unlimited retries with `OperationMaxRetries: -1` for critical operations
- Always increment attempt counters before backoff calculation

## Load Testing and Benchmarking

### Load Generator Patterns
- Support both HTTP and gRPC load generation scripts
- Use UUID generation for unique resource identifiers per test run
- Implement create-only mode (`-c` flag) for sustained load scenarios
- Include health checks (`/livez`) before each operation cycle

**Rules:**
- Always verify service health before generating load
- Use parameterizable intervals and iteration counts
- Support both protocols (HTTP/gRPC) for comprehensive testing
- Generate unique identifiers per operation to avoid conflicts

## Memory and Resource Optimization

### PProf Integration
- Disabled by default, explicit enablement required
- Bound to configurable address (default: `0.0.0.0:5000`)
- Full endpoint coverage: heap, goroutine, CPU, trace, mutex, block profiles
- Security warning: never expose in production environments

**Rules:**
- Only enable pprof in development or controlled debugging environments
- Bind to `127.0.0.1` for local-only access in production debugging
- Disable immediately after collecting necessary profiling data
- Use firewall rules to restrict access to pprof endpoints