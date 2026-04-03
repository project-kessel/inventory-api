# API Contract Guidelines - Kessel Inventory API

## Repository-Specific Patterns

### Threshold Constants
- `BULK_OPERATION_THRESHOLD = 5` - Use bulk operations for >5 individual operations  
- `STREAMING_THRESHOLD = 100` - Use server streaming for list operations returning >100 items

### Package Structure and Naming
- All protobuf definitions use package `kessel.inventory.{version}`
- Go package: `github.com/project-kessel/inventory-api/api/kessel/inventory/{version}`
- Java package: `org.project_kessel.api.inventory.{version}`
- Resource types use snake_case (e.g., `k8s_cluster`, `notifications_integration`)
- Reporter types use lowercase alphanumeric with underscores (e.g., `hbi`, `acm`, `acs`)

### Versioning Strategy
- Main API in `v1beta2` - actively developed
- Health endpoints in stable `v1`
- Breaking changes tracked via buf.build BSR: `buf.build/project-kessel/inventory-api`
- Use `buf breaking --against` for compatibility checks

### HTTP/gRPC Mapping Conventions
- Most endpoints use POST, health checks use GET
- URL pattern: `/api/kessel/{version}/{operation}`
- Bulk operations append "bulk" to endpoint name
- Self operations append "self" to endpoint name
- Resource CRUD uses `/resources` path with appropriate HTTP methods

### Request Validation Patterns
```proto
// Required fields
ResourceReference object = 1 [(buf.validate.field).required = true];

// String validation with regex
string type = 2 [(buf.validate.field).string = {min_len: 1}, (buf.validate.field).string.pattern = "^[A-Za-z0-9_-]+$"];

// Enum validation
WriteVisibility write_visibility = 6 [(buf.validate.field).enum.defined_only = true];

// Array validation
repeated CheckBulkResponsePair pairs = 1 [(buf.validate.field).repeated.min_items = 1];

// Pagination limits
uint32 limit = 1 [(buf.validate.field).uint32 = {gt: 0}];
```

### Consistency Model Implementation
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

### Resource Reference Structure
```proto
message ResourceReference {
  string resource_type = 1;     // Validated: ^[A-Za-z0-9_]+$
  string resource_id = 2;       // Min length: 1
  optional ReporterReference reporter = 3;  // For multi-reporter resources
}
```

### Bulk Operation Guidelines
- Bulk endpoints maintain request/response order
- Use bulk operations for >BULK_OPERATION_THRESHOLD individual operations
- Bulk responses include per-item status
- All bulk operations require min_items = 1 validation

### Pagination Implementation
```proto
message RequestPagination {
  uint32 limit = 1;                      // Must be > 0
  optional string continuation_token = 2; // Opaque token
}
```
- Use continuation tokens, not offset-based pagination
- Streaming operations for large result sets (>STREAMING_THRESHOLD items)

### Test Structure Requirements
Protobuf test files include:
- `TestXxx_FullRoundTrip()` - JSON marshal/unmarshal
- `TestXxx_Reset()` - Reset method validation  
- Additional proto-related tests for String representation, interface compliance, reflection capabilities, nil pointer safety, and zero value handling

### Code Generation Setup
Required buf.gen.yaml plugins:
```yaml
plugins:
  - local: protoc-gen-go           # Core Go types
  - local: protoc-gen-go-grpc      # gRPC server/client
  - local: protoc-gen-go-http      # Kratos HTTP bindings
  - remote: buf.build/community/google-gnostic-openapi  # OpenAPI spec
```

### Schema Organization
Resource schemas stored in `data/schema/resources/{resource_type}/`:
- `config.yaml` - Resource type configuration
- `common_representation.json` - Shared attributes
- `reporters/{reporter_type}/` - Reporter-specific schemas

### Service Documentation Standards
Every RPC method must include:
- Purpose statement answering "What does this do?"
- Question format: "Does subject X have relation Y on object Z?"
- Common use cases (3-5 examples)
- Consistency guarantees
- Performance characteristics for bulk operations

### Error Handling Patterns
- Use `google.rpc.Status` for error responses
- Return `codes.InvalidArgument` for validation failures
- Include detailed error messages for debugging
- Preserve error context through service layers

### Resource Type Validation

**Context-specific patterns** (different validation rules apply in different contexts):

**Resource Reporting Context** (`ReportResourceRequest`):
- Resource type allows dashes: `^[A-Za-z0-9_-]+$`
- Reporter type allows dashes: `^[A-Za-z0-9_-]+$`

**Resource Reference Context** (`ResourceReference`, `ReporterReference`):
- Resource type underscores only: `^[A-Za-z0-9_]+$` 
- Reporter type underscores only: `^[A-Za-z0-9_]+$`

**Rule**: Use dashes for reporting/input, normalize to underscores for references/storage
- All identifiers require min_len = 1

### Streaming Response Guidelines
- Use server streaming for list operations returning >STREAMING_THRESHOLD items
- Include pagination in streaming requests
- Maintain consistency guarantees across stream chunks
- Handle client disconnection gracefully

### Dependency Management
- googleapis for HTTP annotations
- protovalidate v0.14.1+ for validation
- Kratos v2 for HTTP transport generation
- Lock dependencies in buf.lock for reproducible builds

These guidelines reflect the actual patterns used in the Kessel Inventory API codebase and should be followed for consistency and maintainability.