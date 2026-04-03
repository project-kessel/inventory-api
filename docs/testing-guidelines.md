# Testing Guidelines for Inventory API

## Test Organization & File Structure

### Test File Naming
- **Unit tests**: `<filename>_test.go` alongside source files  
- **E2E tests**: Located in `test/e2e/` directory
- **Generated protobuf tests**: `*.pb_test.go` for message validation
- **Integration tests**: Mix unit and real components in same file structure

### Test Execution Patterns
```bash
# Full test suite (excludes slow e2e by default)  
make test

# E2E tests in Kind cluster
make inventory-up-kind && make check-e2e-tests

# Coverage reporting
make test-coverage
```

## Core Testing Infrastructure

### Database Testing
- **In-memory SQLite**: Use `testutil.NewSQLiteTestDB(t, cfg)` for unit tests
- **Unique databases**: Each test gets isolated DB via test name + timestamp
- **Migrations**: Run `data.Migrate(db, nil)` after DB creation for consistent schema
- **Pattern**: `db := setupInMemoryDB(t)` helper in most test files

### Repository Testing Patterns
```go
// Contract testing - test both real and fake implementations
implementations := []struct {
    name string
    repo func() bizmodel.ResourceRepository
}{
    {"Real Repository", func() { return NewResourceRepository(db, tm, publisher) }},
    {"Fake Repository", func() { return NewFakeResourceRepository() }},
}
```

### Dual Protocol Service Testing
- **Framework**: `testframework_test.go` enables testing both gRPC and HTTP
- **Entry point**: Use `runServerTest(t, factory)` for service layer tests
- **Request building**: `withBody(req, Check, httpEndpoint("POST /path"))`
- **Assertions**: `Assert(t, res, requireError(codes.InvalidArgument))`
- **Response extraction**: `resp := Extract(t, res, expectSuccess(func() *pb.Response{}))`

### Mock Management
- **Central location**: `internal/mocks/mocks.go` contains all mock types
- **Testify/mock**: Preferred mocking framework with expectation verification
- **Fake implementations**: 
  - `NewFakeResourceRepository()` - in-memory resource storage
  - `NewFakeTransactionManager()` - no-op transaction handling
  - `FakeMetricsCollector` - metrics capture for assertions

## Test Data & Fixtures

### Fixture Pattern (Usecase Layer)
```go
// Fluent test data builders
cmd := fixture(t).Basic("host", "hbi", "instance-1", "host-1", "workspace-1")
cmd := fixture(t).Updated(resourceType, reporterType, instance, id, workspace)  
cmd := fixture(t).WithTransactionId(params..., "tx-123")
```

### E2E Test Data
- **JSON fixtures**: `data/testData/v1beta2/*.json` for realistic requests
- **Environment driven**: Uses env vars for endpoints and credentials
- **Database integration**: Real Postgres connection for E2E validation

### Schema Testing
- **In-memory repository**: `data.NewInMemorySchemaRepository()` for tests
- **Validation schemas**: JSON Schema strings embedded in test setup
- **Resource/reporter combinations**: Pre-configure valid type mappings

## Authorization Testing Patterns

### Authentication Stubs
```go
// Allow authentication
auth := &StubAuthenticator{
    Claims: &authnapi.Claims{SubjectId: "test-user"},
    Decision: authnapi.Allow,
}

// Deny authentication  
auth := &DenyAuthenticator{}
```

### Authorization Testing
- **Simple authorizer**: `authz.NewSimpleAuthorizer()` for grant-based testing
- **Allow all**: `&allow.AllowAllAuthz{}` to bypass authz in logic tests
- **Mock authorizer**: `mocks.MockAuthz` with expectations

### Meta-Authorization
- **Permissive**: `PermissiveMetaAuthorizer{}` allows all operations
- **Recording**: Custom authorizers that track which relations were checked
- **Denying**: `DenyingMetaAuthorizer{}` to test denial paths

## Test Structure & Organization

### Table-Driven Tests
```go
tests := []struct {
    name        string
    input       InputType
    expectError string
}{
    {"valid case", validInput, ""},
    {"invalid case", invalidInput, "expected error message"},
}
```

### Test Phases (Complex Lifecycle Tests)
- **Phase naming**: Clear comments like `// 1. REPORT NEW` 
- **State verification**: Assert state after each lifecycle step
- **Generation/version tracking**: Critical for resource versioning logic

### Error Testing Patterns
- **gRPC codes**: Use `codes.InvalidArgument`, `codes.NotFound`, etc.
- **Status verification**: Check both gRPC status and HTTP status codes
- **Error message validation**: Assert on specific error text for UX

## Resource Lifecycle Testing

### Versioning Patterns
Test these state transitions extensively:
- **Generation**: Increments when resource revived from tombstone
- **RepresentationVersion**: Increments on each data change  
- **Tombstone**: Tracks deletion state
- **Idempotency**: Same transaction ID = no version changes

### Transaction ID Testing
- **Idempotency**: Same transaction ID should not create new versions
- **Nil handling**: Test both explicit and generated transaction IDs
- **Lifecycle**: Report → Update → Delete → Report patterns

## Consistency & Feature Flag Testing

### Read-After-Write Testing
- **Feature flags**: Test both enabled/disabled states
- **Consistency tokens**: Test all consistency levels
- **Override behavior**: Test bypass flags for consistency

### Configuration Testing
```go
usecaseConfig := &UsecaseConfig{
    ReadAfterWriteEnabled: true,
    DefaultToAtLeastAsAcknowledged: false,
}
```

## Parallel & Race Condition Testing

### Consumer Testing
- **Race condition tests**: `race_condition_test.go` patterns
- **Mock Kafka**: Use `mocks.MockConsumer` for message simulation
- **Database state**: Verify concurrent operations don't corrupt data

### Test Execution
- **Race detection**: All tests run with `-race` flag in CI
- **Count verification**: `-count=1` prevents test caching issues
- **Short flag**: `-short` skips slow integration tests

## E2E & Integration Patterns

### Environment Configuration
```bash
# Required E2E environment variables
INV_HTTP_URL=localhost:8081
INV_GRPC_URL=localhost:9081
POSTGRES_USER/PASSWORD/HOST/PORT/DB
KAFKA_BOOTSTRAP_SERVERS
```

### Container Testing
- **Kind clusters**: Full environment with Postgres, Kafka, SpiceDB
- **Docker builds**: E2E tests run in containerized environment
- **Service discovery**: Tests adapt to running vs local development

### Migration Testing
- **Advisory locks**: Test concurrent migration safety
- **Timestamp validation**: Migration IDs must be valid timestamps
- **Rollback safety**: Verify migration ordering and conflicts

## Common Anti-Patterns to Avoid

1. **Database sharing**: Each test should get isolated database
2. **Global state**: Avoid package-level variables that leak between tests
3. **Time dependencies**: Use mocked time where possible
4. **External dependencies**: Mock all external services in unit tests
5. **Transaction leakage**: Ensure transactions are properly committed/rolled back

## Coverage Targets & Metrics

- **Unit test coverage**: Reported automatically via `make test`
- **Integration coverage**: E2E tests validate full request flows  
- **Mock verification**: Call `mock.AssertExpectations(t)` in teardown when using mocks
- **Metrics verification**: Use `metricscollector.Get*Count()` for event tracking

This repository prioritizes thorough lifecycle testing, dual-protocol validation, and comprehensive authorization testing over simple unit test coverage numbers.