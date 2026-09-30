# Pprof Profiling Server

This package provides a pprof profiling server for the inventory-api application to help with performance analysis and debugging.

## Configuration

The pprof server is **disabled by default** and must be explicitly enabled via command-line flags or configuration.

### Command Line Flags

- `--server.pprof.enabled`: Enable the pprof profiling server (default: `false`)
- `--server.pprof.port`: Port for the pprof server (default: `5000`)
- `--server.pprof.addr`: Address for the pprof server to bind to (default: `0.0.0.0`)

### Example Usage

To start the inventory server with pprof enabled:

```bash
./bin/inventory-api serve --server.pprof.enabled
```

To specify a custom port:

```bash
./bin/inventory-api serve --server.pprof.enabled --server.pprof.port 6060
```

## Accessing Profiling Data

Once the pprof server is running, you can access various profiling endpoints:

### Available Endpoints

- `http://localhost:5000/debug/pprof/` - Index page with links to all profiles
- `http://localhost:5000/debug/pprof/heap` - Heap memory profile
- `http://localhost:5000/debug/pprof/goroutine` - Goroutine profile
- `http://localhost:5000/debug/pprof/threadcreate` - Thread creation profile
- `http://localhost:5000/debug/pprof/block` - Block contention profile
- `http://localhost:5000/debug/pprof/mutex` - Mutex contention profile
- `http://localhost:5000/debug/pprof/profile` - CPU profile (30 seconds by default)
- `http://localhost:5000/debug/pprof/trace` - Execution trace

### Using the Go Tool

You can use the `go tool pprof` command to analyze profiles:

#### CPU Profile (30 seconds)
```bash
go tool pprof http://localhost:5000/debug/pprof/profile
```

#### Heap Profile
```bash
go tool pprof http://localhost:5000/debug/pprof/heap
```

#### Goroutine Profile
```bash
go tool pprof http://localhost:5000/debug/pprof/goroutine
```

#### Interactive Web UI
```bash
go tool pprof -http=:8080 http://localhost:5000/debug/pprof/heap
```

### Capturing Memory Profiles

To capture a memory profile for later analysis:

```bash
curl http://localhost:5000/debug/pprof/heap > heap.prof
go tool pprof heap.prof
```

Or open it directly in the web UI:

```bash
go tool pprof -http=:8080 heap.prof
```

## Security Considerations

**Important**: The pprof server should only be enabled in development or controlled environments. It exposes sensitive information about our application's runtime and should NOT be exposed to the public internet in production.

In production, consider:
- Using firewall rules to restrict access to the pprof port
- Binding to `127.0.0.1` for local-only access
- Only enabling it when actively debugging performance issues
- Disabling it immediately after collecting necessary data

## Example: Investigating High Memory Usage

1. Start the server with pprof enabled:
   ```bash
   ./bin/inventory-api serve --server.pprof.enabled
   ```

2. Generate some load on the application

3. Capture a heap profile:
   ```bash
   go tool pprof -http=:8080 http://localhost:5000/debug/pprof/heap
   ```

4. Use the web UI to analyze memory allocations, look at flame graphs, and identify memory leaks
