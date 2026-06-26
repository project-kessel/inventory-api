# E2E and Integration Test Guidelines

This file covers the E2E and integration tests under `test/`. For layer-specific testing patterns, see the `GUIDELINES.md` file in each layer's directory. For cross-cutting test utilities, see `internal/GUIDELINES.md`.

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

## E2E Test Data
- **JSON fixtures**: `data/testData/v1beta2/*.json` for realistic requests
- **Environment driven**: Uses env vars for endpoints and credentials
- **Database integration**: Real Postgres connection for E2E validation

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

### E2E Testing Setup
- **Kafka consumer tests** with schema validation against CloudEvents specification
- **Environment configuration** via env vars with sensible defaults
- **Topic creation** using admin client before test execution
- **Message filtering**: Skip delete tombstones for event schema validation

### Integration Test Patterns
```go
// Standard test timeout for async operations
ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
defer cancel()
```

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

## Parallel & Race Condition Testing

### Test Execution
- **Race detection**: All tests run with `-race` flag in CI
- **Count verification**: `-count=1` prevents test caching issues
- **Short flag**: `-short` skips slow integration tests
