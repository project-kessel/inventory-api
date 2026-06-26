# Presentation Layer Guidelines (Server Setup)

This file covers server setup under `internal/server/`. For service implementations, see `internal/service/GUIDELINES.md`. For middleware, see `internal/middleware/GUIDELINES.md`. For cross-cutting concerns, see `internal/GUIDELINES.md`.

## TLS and Transport Security

### Client TLS Configuration
- Support both secure and insecure clients via `InsecureClient` flag
- Use `util.NewClient(insecure bool)` for consistent HTTP client creation
- For insecure mode: set `InsecureSkipVerify: true` in TLS config
- Production deployments should enforce TLS verification

### Certificate Management
- Load client certificates from environment variables for E2E testing
- Support CA certificate validation with custom root cert pools
- Handle certificate loading errors gracefully with informative logging

## Custom Stream Metrics
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

## PProf Integration
- Disabled by default, explicit enablement required
- Bound to configurable address (default: `0.0.0.0:5000`)
- Full endpoint coverage: heap, goroutine, CPU, trace, mutex, block profiles
- Security warning: never expose in production environments

**Rules:**
- Only enable pprof in development or controlled debugging environments
- Bind to `127.0.0.1` for local-only access in production debugging
- Disable immediately after collecting necessary profiling data
- Use firewall rules to restrict access to pprof endpoints
