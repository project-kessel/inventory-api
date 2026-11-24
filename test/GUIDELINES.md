# E2E and Integration Test Guidelines

This file covers the E2E and integration tests under `test/`. For layer-specific testing patterns, see the `GUIDELINES.md` file in each layer's directory. For cross-cutting test utilities, see `internal/GUIDELINES.md`.

## Testing Philosophy

**Prefer test-driven development (TDD).** Write tests first to define the expected behavior, then write the implementation to make them pass. TDD produces cleaner interfaces, better coverage, and catches design problems early. When adding a new feature or fixing a bug, start by writing a failing test that captures the requirement, then implement the minimum code to pass it.

We follow a **no-mocks** approach to testing. This means no "method-verifying" mocks (e.g., testify/mock `On(...).Return(...)` patterns that assert how methods are called). This is not to be confused with dummies or stubs, which can be perfectly fine in moderation.

The preference order for test doubles is:

1. **Use the real instance.** If an object is not coupled to external I/O, there is no reason not to reuse it. It is the least work and the best coverage.
2. **Use the real database adapter for persistence.** Resource use-case, service, consumer, and repository tests use migrated databases and the real transaction manager, rather than reimplementing storage semantics in a fake. PostgreSQL is the default; SQLite is an explicit fallback.
3. **Use stubs or dummies** judiciously when the interaction is trivial.

### Hermetic Testing

When external dependencies are needed, leverage testcontainers to download and run them locally. Business logic that uses resource persistence must exercise the real repository and transaction manager too: an in-memory reimplementation can hide bugs in queries, constraints, version selection, and transaction behavior. Relations behavior must also use the real SpiceDB adapter against `serve-testing`, rather than reimplementing tuple storage, permission evaluation, or fencing. Keep pure domain tests free of I/O, and reuse production in-memory adapters such as the schema repository where applicable.

For further reading on the principles behind this approach, see: [The secret world of testing without mocking](https://www.alechenninger.com/2020/11/the-secret-world-of-testing-without.html) by Alec Henninger.

### Database Backend Selection

Tests that verify database behavior (repositories, migrations, constraints, transaction handling) run against **PostgreSQL by default**, because that is what production runs and SQLite cannot reproduce PostgreSQL-specific behavior such as JSONB constraints, advisory locks, and transaction conflicts (RHCLOUD-51816). Tests of business logic that use resource persistence also use the real database adapter; SQLite remains available as an explicit persistence fallback. `TEST_DB=sqlite` intentionally skips tests that depend on SpiceDB, even when a container runtime is available, so SQLite tests need no runtime. The default/PostgreSQL suite requires a working runtime and must fail, not automatically skip tests, when it is unavailable.

Select the backend with `TEST_DB`:

```bash
# PostgreSQL via Testcontainers (default; needs Docker or Podman)
make test                                   # short-mode suite, database tests on PostgreSQL
TEST_DB=postgres ./scripts/testcontainers-run.sh go test ./internal/data/ -count=1 -race -short

# In-memory SQLite persistence (always skips SpiceDB, no runtime required)
make test-sqlite                            # intentionally skips SpiceDB-dependent tests
TEST_DB=sqlite go test ./internal/data/ -count=1 -race -short
```

Both Make targets run with `-short` and explicitly skip the E2E/Kafka test patterns `TestInventoryAPIGRPC_*`, `TestInventoryAPIHTTP_*`, and `Test_ACMKafkaConsumer`; neither runs the full test suite. The SQLite fallback also skips PostgreSQL-specific tests. `make test-sqlite` runs Go tests directly with `TEST_DB=sqlite`, without the container wrapper. The wrapper (including `make test`) always requires a runtime, invalid `CONTAINER_RUNTIME` selections remain errors, and command failures retain their exit status. Run the hermetic wrapper regression checks with `sh scripts/testcontainers-run_test.sh`.

`scripts/testcontainers-run.sh` detects Docker contexts and rootless Podman sockets and sets `DOCKER_HOST` accordingly; it disables Ryuk for Podman, so test binaries must stop their shared containers from `TestMain` via `testutil.TerminateSharedPostgres()`. `TEST_POSTGRES_IMAGE` (default `postgres:17.8`) overrides the container image. The container runs `wal_level=logical`, matching production's Debezium outbox pipeline, so tests exercise the real WAL outbox publisher (`pg_logical_emit_message`) — `outboxeventsrepository_test.go` verifies its output through a logical replication slot. Test helpers wire the WAL publisher on PostgreSQL and the no-op publisher on SQLite.

The PostgreSQL backend (in `internal/testutil`, see `postgres_testdb.go`) starts **one container per test binary**, migrates a **template database once per distinct migrate function**, and copies that template into a **private database per test** — isolation without per-test migration cost. Databases are dropped with `WITH (FORCE)` on test cleanup. In keep-on-failure mode, database drop decisions wait until `TestMain` cleanup after `m.Run`, so even failures in cleanups registered before the fixture preserve their test's data. Passing databases are still dropped before retaining a failed run's container.

When adding database-backed tests:

- Get a migrated database through `testutil.NewTestDB(t, cfg, migrate)`, passing a **package-level** migrate function so its template is reused. Packages without schema needs (e.g. transaction-manager behavior tests) pass `nil`.
- Use `testlifecycle.Run(m, testutil.TerminateSharedPostgres)` from `internal/testutil/testlifecycle` in a package-local `TestMain`, passing the returned code to `os.Exit`. The runner records test failures for retention, attempts all supplied cleanups, reports errors, and fails successful runs if cleanup fails without replacing an existing failing exit status. This is normal-exit cleanup, not signal handling.
- Call repository methods with the returned handle as before; tests do not need to know which backend is active.
- Provision expensive fixtures explicitly only when the tested path reaches them. Validation, authentication-denial, and mapping tests should leave unused collaborators nil or use a fail-fast dummy rather than acquire a database or SpiceDB fixture. The shared PostgreSQL and relations helpers start containers lazily; the presence of `TestMain` does not start them.
- Branch on `testutil.UsingPostgres()` only for behavior that cannot be expressed on both backends, and document why in a comment. Examples: capping lock waits with `SET LOCAL lock_timeout` where SQLite would fail fast instead of blocking (`resource_repository_test.go`), asserting the real isolation level (`transaction_manager_test.go`), or asserting PostgreSQL-specific query results that SQLite cannot run (`cmd/jobs/metrics_collect_job_test.go`).

Tests that deliberately remain on SQLite verify SQLite-specific fallback behavior itself: `migrations/locks_test.go`'s `TestWithAdvisoryLock_Sqlite*` (no locking outside PostgreSQL). Portable database tests run on both backends, with PostgreSQL the default. PostgreSQL-specific tests (advisory locks, real serialization conflicts, WAL decoding, and container cleanup) explicitly skip the SQLite fallback.

CI runs `make test` on every pull request (see `build-test.yml`).

### Shared Container Lifecycle (Required)

Every test package that uses the shared PostgreSQL or SpiceDB fixtures must
register the matching cleanup in a package-local `TestMain`. Otherwise containers
can leak when Ryuk is disabled (including our Podman wrapper and retention mode).
Register **only the resources the package uses**:

```go
package example_test

import (
    "os"
    "testing"

    "github.com/project-kessel/inventory-api/internal/testutil"
    relationstest "github.com/project-kessel/inventory-api/internal/testutil/relations"
    "github.com/project-kessel/inventory-api/internal/testutil/testlifecycle"
)

func TestMain(m *testing.M) {
    os.Exit(testlifecycle.Run(m,
        relationstest.TerminateSharedSpiceDB,
        testutil.TerminateSharedPostgres,
    ))
}
```

- `testutil.NewTestDB`, `NewPostgresTestDB`, and `PostgresTestDSN` require
  `testutil.TerminateSharedPostgres`, even when developing with `TEST_DB=sqlite`.
- `relationstest.New` requires `relationstest.TerminateSharedSpiceDB`.
- Database-only packages omit the SpiceDB import and cleanup; SpiceDB-only
  packages omit the root `testutil` import and PostgreSQL cleanup.
- Keep the returned exit code: use `os.Exit(testlifecycle.Run(...))`, or return
  the runner's code from a custom lifecycle helper to `TestMain`'s `os.Exit`.
- Cleanup is package-wide, but startup remains lazy. Tests that never request a
  fixture do not start its container.

`test/architecture` enforces this convention in the normal `go test ./...` suite,
so both `make test` and `make test-sqlite` check it. Run the guard alone without
containers:

```bash
go test ./test/architecture -run '^TestSharedContainerLifecycle$' -count=1
```

A failure names the missing registration and prints a tailored snippet, e.g.:

```text
example/database_test.go:12: package example uses shared PostgreSQL fixtures but TestMain does not register testutil.TerminateSharedPostgres with testlifecycle.Run.
Add or update main_test.go (import os, testing, and the helper packages):

func TestMain(m *testing.M) {
    os.Exit(testlifecycle.Run(m, testutil.TerminateSharedPostgres))
}

See test/GUIDELINES.md#shared-container-lifecycle-required for imports, examples, and cleanup requirements.
```

The guard resolves named import aliases and follows named function references
through local and imported helpers. Cleanup registration must be in a
`testlifecycle.Run` call reachable through named calls from `TestMain`; mentioning
cleanup elsewhere or putting it in an unused closure does not count. Use named
imports for fixture and lifecycle packages, and pass cleanup functions directly
to the runner. This is a bounded source-level convention check, not a proof of
arbitrary control flow, interface/method dispatch, reflection, or dynamically
assembled cleanup lists. It does not cover raw Testcontainers or independently
owned container helpers; new shared fixture APIs must be added to its rules.

The guard discovers the nearest repository root by walking up to `go.mod`,
derives the module import prefix from its module directive, and verifies that
this documentation file and lifecycle heading exist. A missing or invalid root
or a stale documentation target fails the check rather than silently scanning
an unrelated directory. Fixture API names remain explicit rule configuration.
Source discovery skips hidden directories, `vendor`, `node_modules`, and
`testdata`.

There are no package-wide exceptions. One declaration-specific PostgreSQL
exception is recorded with a reason in
`test/architecture/container_lifecycle_check_test.go`:

- `internal/testutil/testdb_test.go:TestTestDBBackend_Invalid` verifies that an
  invalid backend panics before a container can start.

Exceptions match an exact file, declaration, and fixture, and do not exempt
callers or other declarations in the same file or package. If a declaration is
renamed, moved, removed, or stops referencing the excepted fixture, the stale
exception fails the guard and must be removed or updated.
The legacy data-layer SpiceDB container remains explicitly owned by that suite;
its shared PostgreSQL fixture is checked normally.

### SpiceDB Relations Testing

`internal/testutil/relations` (package `relationstest`) provides the production
SpiceDB adapter backed by SpiceDB's in-memory `serve-testing` server. It starts
one container lazily per test binary, and `relationstest.New(t)` uses a unique
bearer token to select a private datastore, including its schema. Connections
are closed by test cleanup; shared containers must be stopped from `TestMain`
by passing `relationstest.TerminateSharedSpiceDB` to `testlifecycle.Run`, which
checks cleanup errors and preserves an already-failing exit status. This cleanup is required with Podman
when Ryuk is disabled.

- Use `relationstest.Grant(t, repo, subjectID, relation, reporterType, resourceType, resourceID)` for direct principal relationships, or write schema-valid tuples through the real repository for computed permissions and workspace relationships.
- The helper embeds a test schema in `internal/testutil/relations/schema.zed`; this tests real SpiceDB semantics but does not assert compatibility with a deployed RBAC schema. Extend the fixture schema explicitly when a test needs more types or relations.
- Default reads are fully consistent so ordinary state assertions do not need sleeps. Explicit consistency arguments retain their meaning. Use returned write/delete tokens for read-after-write tests.
- An `at_least_as_fresh` token is a lower bound, not a request for a historical snapshot. Do not assert that an older token guarantees stale permission results.
- Exercise fencing errors by acquiring the same lock again and attempting a write with the old token. Use narrow fault-injection stubs only for conditions that cannot be produced deterministically by the real server.
- SpiceDB tests run in short mode too, except when `TEST_DB=sqlite` explicitly disables them. `relationstest.New(t)` intentionally skips dependent tests on SQLite regardless of runtime availability. Otherwise, the container-provider health probe, SpiceDB startup, and schema failures are fatal, not skips. Run the default/PostgreSQL suite through `scripts/testcontainers-run.sh` to configure the required runtime; `make test-sqlite` bypasses the wrapper and never starts SpiceDB.

```bash
./scripts/testcontainers-run.sh go test -count=1 -race -short \
  ./internal/testutil/relations ./internal/biz/usecase/resources \
  ./internal/service/resources ./internal/consumer ./internal/data/health
```

The helper shares the pinned image version used by the existing data-layer
SpiceDB integration tests. The latter retain their separate bootstrap schema
and existing non-short execution mode, but also intentionally skip on
`TEST_DB=sqlite`, even without `-short`.

### Test Container Debugging

The shared PostgreSQL and SpiceDB helpers and the legacy non-short data-layer
SpiceDB helper use `internal/testutil/containerdebug`. See the
[test-container debugging guide](../docs/dev-guides/test-container-debugging.md)
for the JSONL registry schema, host `psql` / Authzed `zed` commands, and cleanup.

- `TEST_CONTAINERS_DEBUG=1` prints nonsecret container endpoints, IDs, fixture associations, and inspect/log commands. Never print tokens, passwords, or credential-bearing DSNs.
- `TEST_CONTAINERS_DEBUG_FILE=<base>` implies debug and exclusively creates `<base>.<binary>.<pid>.jsonl` with mode `0600`, including full credentials. Files persist after cleanup; delete them manually and treat them as secrets.
- `TEST_CONTAINERS_KEEP_ON_FAILURE=1` implies debug and retains shared containers after observed fixture or package failures. Only failed tests' private PostgreSQL databases are kept; successful private databases are still dropped. SpiceDB's in-memory fixture datastores remain with the retained server. Successful runs still terminate containers normally.
- Call `containerdebug.Configure()` before **any** Testcontainers operation (including health probes, configuration reads, session-ID and label helpers), so keep mode disables Ryuk before runtime configuration is cached. The wrapper also disables Ryuk in the launching environment for keep mode, covering package initializers that run before `TestMain`; set `TESTCONTAINERS_RYUK_DISABLED=true` yourself if bypassing it. A late `Configure()` cannot undo cached configuration or a running Ryuk. Propagate configuration/registry errors rather than silently losing diagnostics.
- Call `containerdebug.Observe(t)` after explicit SQLite/short-mode skip guards and before fixture setup/cleanup. Register actual container IDs and host endpoints, and register fixture tokens before SpiceDB repository/schema initialization. PostgreSQL cleanup uses `PreserveTest(t)` before dropping each private database.
- In `TestMain`, use `testlifecycle.Run(m, cleanupFunctions...)` to record `m.Run()`'s status before shared-container cleanup, including mixed packages whose failing tests may not own a fixture. Record custom setup failures with `containerdebug.RecordResult(code)` too. Container owners check `KeepOnFailure()` and report retained IDs with `Retained(id)` instead of terminating them; otherwise preserve normal strict cleanup and failure handling.
- Use the wrapper with `-v -count=1` so successful-package stdout is visible and cached results cannot hide current containers. Do not add `-short` when debugging the legacy data-layer SpiceDB suite. SQLite always excludes SpiceDB, regardless of debug settings.
- Stop retained containers explicitly with `podman rm -f -v <container-id>` to remove anonymous PostgreSQL volumes too; credential registry files require separate manual deletion. Keep mode disables Ryuk even on successful runs, so normal `TestMain` cleanup must remain in place.

#### Measured overhead (2026-11-10)

Compared against commit `78f93947`, using rootless Podman, PostgreSQL, the pinned
SpiceDB `v1.47.1` image, and `-race -short -count=1`. Three alternating
before/after invocations used the same five packages, default Go package
parallelism, warmed build caches and locally cached images. Each invocation
started and stopped fresh shared containers; Go test-result caching was disabled.
These are local measurements, not CI performance guarantees or cold-pull timings.

| Package / measurement | Before median | After median | Change |
|---|---:|---:|---:|
| Resource use cases | 13.155 s | 13.809 s | +0.654 s |
| Resource services | 25.994 s | 27.976 s | +1.982 s |
| Consumer | 20.769 s | 21.787 s | +1.018 s |
| Data | 17.300 s | 17.456 s | +0.156 s |
| Data health | 9.111 s | 9.574 s | +0.463 s |
| Whole five-package command, wall clock | 27.947 s | 29.979 s | **+2.032 s (+7.3%)** |

The wall-clock samples were 27.947/28.016/27.477 s before and
29.756/30.422/29.979 s after. Package durations overlap; do not sum them. The
existing data package is included as a comparison: its non-short SpiceDB tests
are unchanged, while the deleted fake's tests are no longer run.

Reproduce the package selection with:

```bash
./scripts/testcontainers-run.sh go test -json -count=1 -race -short \
  ./internal/biz/usecase/resources ./internal/service/resources \
  ./internal/consumer ./internal/data ./internal/data/health
```

The new helper's own integration tests passed separately and in the full suite.
A repository-wide short run with `-race -count=1` and the usual E2E/Kafka
exclusions passed in 36.3 s wall clock (without coverage instrumentation); this
was a single validation run, not a repeated whole-repository comparison.

## Test Organization & File Structure

### Test File Naming
- **Unit tests**: `<filename>_test.go` alongside source files  
- **E2E tests**: Located in `test/e2e/` directory
- **Generated protobuf tests**: `*.pb_test.go` for message validation
- **Integration tests**: Mix unit and real components in same file structure

### Test Execution Patterns
```bash
# Short-mode suite (excludes E2E and Kafka test patterns)
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
5. **Reimplementing persistence in tests**: Use the real resource repository and transaction manager even when testing business logic. Fakes with their own storage logic can pass while production behavior is broken.
6. **Faking what shouldn't be faked**: Don't write a fake for something that inherently requires the real thing (e.g., Postgres SQL semantics). Use testcontainers or in-memory SQLite instead.
7. **Transaction leakage**: Ensure transactions are properly committed/rolled back

## Coverage Targets & Metrics

- **Unit test coverage**: Reported automatically via `make test`
- **Integration coverage**: E2E tests validate full request flows  
- **Persistence coverage**: Exercise resource behavior through real database adapters, including commit, rollback, constraints, and representation history
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
