# Testing Guidelines for Inventory API

## Testing Philosophy

**Prefer test-driven development (TDD).** Write tests first to define the expected behavior, then write the implementation to make them pass. TDD produces cleaner interfaces, better coverage, and catches design problems early. When adding a new feature or fixing a bug, start by writing a failing test that captures the requirement, then implement the minimum code to pass it.

We follow a **no-mocks** approach to testing. This means no "method-verifying" mocks (e.g., testify/mock `On(...).Return(...)` patterns that assert how methods are called). This is not to be confused with dummies or stubs, which can be perfectly fine in moderation.

The preference order for test doubles is:

1. **Use the real instance.** If an object is not coupled to external I/O, there is no reason not to reuse it. It is the least work and the best coverage.
2. **Use a fake.** In-memory fakes are a useful feature of the application ("Kessel in a box"), so the investment pays for itself quickly. When implementing fakes (or any second implementation of an interface), define a set of **contract tests** at the interface layer first.
3. **Use stubs or dummies** judiciously when the interaction is trivial.

### Hermetic Testing

When external dependencies are needed, leverage testcontainers to download and run them locally. This should only be for when it is essential. For example, we can't test a `PostgresStore` without a Postgres — writing a "fake" Postgres is absurd. But if you need to test business logic that involves a repository, using a real Postgres is overkill. Just use the in-memory fake (e.g., a custom in-memory implementation, or SQLite with an in-memory database).

For further reading on the principles behind this approach, see: [The secret world of testing without mocking](https://www.alechenninger.com/2020/11/the-secret-world-of-testing-without.html) by Alec Henninger.

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

### Test Double Strategy

**Prefer real instances and fakes over mocks.** The codebase provides a rich set of in-memory fakes that serve as both test infrastructure and a feature of the application itself:

| Implementation | Location | Purpose |
|----------------|----------|---------|
| `NewFakeResourceRepository()` | `internal/data/fake_resource_repository.go` | Full in-memory resource storage with mutex and transaction ID tracking |
| `NewFakeTransactionManager()` | `internal/data/fake_transaction_manager.go` | No-op transaction handling for tests that don't need real DB transactions |
| `NewFakeMetricsCollector()` | `internal/metricscollector/fake_metricscollector.go` | No-op OTEL counters for tests that need a `MetricsCollector` shape |
| `NewSimpleRelationsRepository()` | `internal/data/relations_simple.go` | In-process Relations API behavior (tuples, checks, snapshots) |

Note: `NewInMemorySchemaRepository()` (`internal/data/schema_inmemory.go`) is the **production** schema store, not a fake. It is used directly in tests as a real instance — a good example of preferring the real thing when no external I/O is involved.

These fakes are not throw-away test code — they are **reusable, evolvable implementations** that encode domain knowledge and can be used to run the service with zero external dependencies.

### Contract Testing

When implementing a fake, define contract tests that run against both the real and fake implementations. This ensures the fake faithfully implements the interface contract.

```go
// Contract testing - test both real and fake implementations
implementations := []struct {
    name string
    repo func() bizmodel.ResourceRepository
}{
    {"Real Repository", func() { return NewResourceRepository(db, tm, publisher) }},
    {"Fake Repository", func() { return NewFakeResourceRepository() }},
}

for _, impl := range implementations {
    t.Run(impl.name, func(t *testing.T) {
        testRepositoryContract(t, impl.repo())
    })
}
```

See `internal/data/resource_repository_test.go` for the canonical example of this pattern.

### Database Testing
- **In-memory SQLite**: Use `testutil.NewSQLiteTestDB(t, cfg)` for tests that need real SQL behavior
- **Unique databases**: Each test gets isolated DB via test name + timestamp
- **Migrations**: Run `data.Migrate(db, nil)` after DB creation for consistent schema
- **Pattern**: `db := setupInMemoryDB(t)` helper in most test files
- **When to use SQLite vs fake**: Use the fake repo for business logic tests. Use SQLite when the test specifically needs SQL/GORM behavior (e.g., testing query construction, migration logic, or constraint handling).

### Dual Protocol Service Testing
- **Framework**: `testframework_test.go` enables testing both gRPC and HTTP
- **Entry point**: Use `runServerTest(t, factory)` for service layer tests
- **Request building**: `withBody(req, Check, httpEndpoint("POST /path"))`
- **Assertions**: `Assert(t, res, requireError(codes.InvalidArgument))`
- **Response extraction**: `resp := Extract(t, res, expectSuccess(func() *pb.Response{}))`

## Test Data & Fixtures

### Fixture Pattern (Usecase Layer)
```go
// Fluent test data builders
// Fluent test data builders (independent examples)
basicCmd := fixture(t).Basic("host", "hbi", "instance-1", "host-1", "workspace-1")
updatedCmd := fixture(t).Updated(resourceType, reporterType, instance, id, workspace)  
txCmd := fixture(t).WithTransactionId(params..., "tx-123")
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

**Relations Repository** (for resource-level authorization):
- **Simple relations repo**: `data.NewSimpleRelationsRepository()` - in-memory implementation with real tuple storage and authorization logic
- **Allow all**: `&data.AllowAllRelationsRepository{}` - bypasses all authorization checks for logic tests

**Meta-Authorization** (for endpoint/CRUD-level authorization):
- **Permissive**: `&PermissiveMetaAuthorizer{}` - allows all operations for golden-path testing
- **Denying**: `&DenyingMetaAuthorizer{}` - denies all operations to test denial paths
- **Simple**: `metaauthorizer.NewSimpleMetaAuthorizer()` - service-based meta authorization with configurable rules

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
- **Database state**: Verify concurrent operations don't corrupt data
- **Kafka events**: Simulate Kafka messages using a fake consumer (see `internal/consumer/*_test.go` for examples)

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

1. **Method-verifying mocks**: Do not use `testify/mock` with `On(...).Return(...)` to verify how methods are called. Use a real instance or a fake instead.
2. **Database sharing**: Each test should get isolated database
3. **Global state**: Avoid package-level variables that leak between tests
4. **Time dependencies**: Use mocked time where possible
5. **Reaching for a real DB when a fake suffices**: If you're testing business logic, use the in-memory fake. Reserve SQLite/Postgres for tests that specifically exercise data layer behavior.
6. **Faking what shouldn't be faked**: Don't write a fake for something that inherently requires the real thing (e.g., Postgres SQL semantics). Use testcontainers or in-memory SQLite instead.
7. **Transaction leakage**: Ensure transactions are properly committed/rolled back

## Coverage Targets & Metrics

- **Unit test coverage**: Reported automatically via `make test`
- **Integration coverage**: E2E tests validate full request flows  
- **Contract test coverage**: Ensure fakes are validated against real implementations via shared contract tests
- **Metrics verification**: Use `metricscollector.Get*Count()` for event tracking

This repository prioritizes thorough lifecycle testing, dual-protocol validation, and comprehensive authorization testing over simple unit test coverage numbers.