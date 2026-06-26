# Application Services Layer Guidelines

This file covers the application services (usecases) under `internal/biz/usecase/`. For domain model guidelines, see `internal/biz/model/GUIDELINES.md`. For cross-cutting concerns, see `internal/GUIDELINES.md`.

## Error Handling

### Application-Layer Errors in `usecase/resources`
- **Authorization errors**: `ErrMetaAuthzContextMissing`, `ErrSelfSubjectMissing`, `ErrMetaAuthorizerUnavailable`, `ErrMetaAuthorizationDenied`
- Re-export domain errors: `ErrResourceNotFound = model.ErrResourceNotFound`

## Authorization

### Bulk Authorization Patterns
- Prefer `CheckBulk` over individual `Check` calls for performance
- Use consistency tokens to maintain read-after-write semantics
- Implement workspace-scoped authorization with RBAC namespace

### Authorization Logging
- Track authorization success/failure counters per method
- Use OpenTelemetry meter: `inventory_relations_api_success/failure`
- Log authorization decisions at Info level with resource details

### Authorization Error Handling
- Increment failure metrics on authorization errors
- Log authorization decisions with resource context
- Handle gRPC errors from relations-api gracefully
- Provide meaningful error messages without exposing internal details

### Authorization Patterns
- **Kessel authz** for production with Relations API integration
- **Allow-all authz** for testing and development scenarios only
  - ⚠️ **Security Warning**: `allow-all` bypasses all authorization checks
  - Should never be used in production environments  
  - Requires explicit configuration and awareness of security implications
- **Subject reference construction** from authenticated identity context

## Health Check Implementation

### Service Health Patterns
- **Livez endpoint**: Simple OK response, configurable logging via `log.livez`
- **Readyz endpoint**: Check backend availability (database, relations-api)
- **Relations health**: Call Relations API `/readyz` endpoint with proper auth
- Use **helper logging** to avoid spam: log disable messages only once

### Database Health
- Check **PostgreSQL connectivity** via GORM connection
- Validate **migration state** if applicable
- Return **structured responses** with status codes (200 for OK)

## Bulk Operations
- Use **CheckBulk/CheckForUpdateBulk** for authorization checks on multiple resources
- Default to **minimize_latency** consistency when no token available
- Use **at_least_as_fresh** consistency when token is present from previous operations

## Transaction Boundary

Always use `HandleSerializableTransaction` for resource CRUD operations. Keep transactions lean — avoid complex multi-table operations in a single transaction. See `internal/data/GUIDELINES.md` for the full transaction management pattern and retry logic.

## Testing

### Fixture Pattern (Usecase Layer)
```go
// Fluent test data builders
// Fluent test data builders (independent examples)
basicCmd := fixture(t).Basic("host", "hbi", "instance-1", "host-1", "workspace-1")
updatedCmd := fixture(t).Updated(resourceType, reporterType, instance, id, workspace)  
txCmd := fixture(t).WithTransactionId(params..., "tx-123")
```

### Authorization Testing Patterns

**Authentication Stubs:**
```go
// Allow authentication
auth := &StubAuthenticator{
    Claims: &authnapi.Claims{SubjectId: "test-user"},
    Decision: authnapi.Allow,
}

// Deny authentication  
auth := &DenyAuthenticator{}
```

**Relations Repository** (for resource-level authorization):
- **Simple relations repo**: `data.NewSimpleRelationsRepository()` - in-memory implementation with real tuple storage and authorization logic
- **Allow all**: `&data.AllowAllRelationsRepository{}` - bypasses all authorization checks for logic tests

**Meta-Authorization** (for endpoint/CRUD-level authorization):
- **Permissive**: `&PermissiveMetaAuthorizer{}` - allows all operations for golden-path testing
- **Denying**: `&DenyingMetaAuthorizer{}` - denies all operations to test denial paths
- **Simple**: `metaauthorizer.NewSimpleMetaAuthorizer()` - service-based meta authorization with configurable rules

### Transaction ID Testing
- **Idempotency**: Same transaction ID should not create new versions
- **Nil handling**: Test both explicit and generated transaction IDs
- **Lifecycle**: Report → Update → Delete → Report patterns

### Consistency & Feature Flag Testing

#### Read-After-Write Testing
- **Feature flags**: Test both enabled/disabled states
- **Consistency tokens**: Test all consistency levels
- **Override behavior**: Test bypass flags for consistency

#### Configuration Testing
```go
usecaseConfig := &UsecaseConfig{
    ReadAfterWriteEnabled: true,
    DefaultToAtLeastAsAcknowledged: false,
}
```
