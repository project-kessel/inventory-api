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

## gRPC Keepalive Compatibility

- Preserve the server enforcement policy in `grpc/config.go`: `MinTime: 30 * time.Second` and `PermitWithoutStream: true`.
- This accepts the SDK keepalive defaults: a 45-second ping interval, a 10-second acknowledgement timeout, and pings without active RPCs. These are client settings, not server-originated ping settings.
- Keep raw gRPC options in `CompletedConfig.GRPCOptions` and merge them with interceptors into a single `kgrpc.Options()` call in `NewWithDeps`; multiple calls replace, rather than append to, the underlying options.
- Coordinate changes with the [shared SDK specification](https://project-kessel.github.io/docs/contributing/client-api/service-version/) and deployed gateways: receiving endpoints must permit idle pings and accept the configured cadence.
- When changing this policy or its wiring, verify a direct local connection using the SDK defaults remains idle for at least three 45-second ping intervals without policy-triggered `GOAWAY` (`ENHANCE_YOUR_CALM` / `too_many_pings`), then completes an RPC without a policy-triggered reconnect.

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
- Bound to configurable address (default: `127.0.0.1:5000`)
- Full endpoint coverage: heap, goroutine, CPU, trace, mutex, block profiles
- Security warning: never expose in production environments

**Rules:**
- Only enable pprof in development or controlled debugging environments
- Bind to `127.0.0.1` for local-only access in production debugging
- Disable immediately after collecting necessary profiling data
- Use firewall rules to restrict access to pprof endpoints
