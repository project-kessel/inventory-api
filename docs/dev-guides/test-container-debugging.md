# Debugging test containers

The PostgreSQL helper (`internal/testutil`), shared SpiceDB relations helper
(`internal/testutil/relations`), and legacy data-layer SpiceDB helper
(`internal/data/spicedb_container.go`) support the same opt-in diagnostics.
PostgreSQL normally shares one container per test binary, with migrated templates
and a private database per test. Both SpiceDB helpers use `serve-testing`: each
repository fixture has a private in-memory datastore selected by its bearer token.
The legacy helper has a separate bootstrap schema and runs only without `-short`.
`TEST_DB=sqlite` always skips SpiceDB and PostgreSQL-specific tests; debugging does
not change those guards or turn runtime/startup/schema failures into skips.

## Enable diagnostics

| Variable | Behavior |
| --- | --- |
| `TEST_CONTAINERS_DEBUG=1` | Print nonsecret container and fixture metadata plus inspect/log commands to stdout. |
| `TEST_CONTAINERS_DEBUG_FILE=/absolute/path/registry` | Also write full connection credentials to a JSONL registry; implies debug output. The parent directory must already exist. |
| `TEST_CONTAINERS_KEEP_ON_FAILURE=1` | Retain shared containers after an observed test or package failure; implies debug output. |

Boolean flags accept Go `strconv.ParseBool` values; invalid values fail setup.
With all options unset, normal cleanup is unchanged. Debug output alone does not
retain anything: successful runs still close connections, drop private PostgreSQL
databases, and terminate containers, including when keep-on-failure is enabled.
The legacy SpiceDB helper also terminates partially started containers on startup
or readiness failure, even in keep mode; these are not usable repository fixtures.
Any failure to remove a partial startup container is reported with its ID.

Run through the runtime-detection wrapper. Use `-v` so Go shows diagnostics even
for passing packages, and `-count=1` to bypass cached test results:

```sh
# Nonsecret stdout only; includes legacy data-layer SpiceDB tests (no -short).
TEST_CONTAINERS_DEBUG=1 ./scripts/testcontainers-run.sh \
  go test -v -race -count=1 ./internal/data

# Credentials registry plus post-failure inspection.
mkdir -p /tmp/inventory-container-debug
TEST_CONTAINERS_DEBUG_FILE=/tmp/inventory-container-debug/registry \
TEST_CONTAINERS_KEEP_ON_FAILURE=1 ./scripts/testcontainers-run.sh \
  go test -v -race -count=1 ./internal/data ./internal/testutil/relations
```

For keep mode, the wrapper sets `TESTCONTAINERS_RYUK_DISABLED=true` in the
launching environment, before the test process starts. This also covers package
initializers that read Testcontainers configuration before `TestMain`.
Helpers must still call `containerdebug.Configure()` **before any Testcontainers
operation**, including provider health probes, runtime configuration reads,
session-ID or label helpers. Testcontainers caches that configuration; calling
`Configure` later cannot undo an already-cached setting or a running Ryuk;
keep-mode setup fails if the effective cached configuration still enables Ryuk.
If bypassing the wrapper, set `TESTCONTAINERS_RYUK_DISABLED=true` in the launching
environment yourself when using keep mode. The Podman wrapper also disables Ryuk
normally; this does not by itself enable retention. TestMain cleanup remains
necessary, and abrupt termination or a crash can leave containers behind when
Ryuk is disabled.

## Registry layout and security

A base path is not the output filename. Each test process creates its own file:

```text
/tmp/inventory-container-debug/
  registry.data.test.<pid>.jsonl
  registry.relations.test.<pid>.jsonl
```

The general form is `<base>.<executable-basename>.<pid>.jsonl`. The actual filename
is printed at startup. Files are created exclusively (`O_EXCL`) with mode `0600`;
existing files are not overwritten. Each line is one JSON object. Empty fields
are omitted:

| `type` | Fields |
| --- | --- |
| `container` | `kind` (`postgres` or `spicedb`), `container_id`, host-mapped `endpoint`; PostgreSQL also has `username`, `password`. |
| `fixture` | `kind`, `container_id`, `test` (the full `t.Name()`); PostgreSQL has `database`, SpiceDB has `token`. |
| `retained` | `container_id`, emitted when shared-container cleanup deliberately retains it. |

Join fixture records to container records by `container_id`, within the same
process file. SpiceDB fixture tokens are registered before repository/schema
setup, so a failed setup can still be inspected. PostgreSQL migration failures
can register the partially initialized template database for diagnosis.

Stdout contains metadata only: **no passwords, tokens, or credential-bearing
DSNs**. Only the explicitly requested registry contains full credentials. Treat
these files as secrets: do not commit them, attach them to tickets, echo their
contents, or enable shell tracing while loading credentials. Registry files
persist even after successful cleanup and must be deleted manually.

## Inspect PostgreSQL from the host

Use `podman inspect <container-id>` and `podman logs <container-id>` from the
nonsecret output to inspect the container. To query a retained failed test's
database, select its exact fixture record and load credentials into environment
variables rather than printing a password or putting it in a DSN/command argument:

```sh
# Substitute the actual registry filename and full test name. Keep tracing off.
set +x
registry=/tmp/inventory-container-debug/registry.data.test.12345.jsonl
test_name=TestExample/failing_case
container_id=$(jq -r --arg test "$test_name" \
  'select(.type == "fixture" and .kind == "postgres" and .test == $test) | .container_id' "$registry" | tail -n 1)
export PGDATABASE=$(jq -r --arg test "$test_name" \
  'select(.type == "fixture" and .kind == "postgres" and .test == $test) | .database' "$registry" | tail -n 1)
endpoint=$(jq -r --arg id "$container_id" \
  'select(.type == "container" and .container_id == $id) | .endpoint' "$registry")
export PGHOST="${endpoint%:*}" PGPORT="${endpoint##*:}"
# Strip JoinHostPort's brackets if the mapped host is IPv6.
PGHOST="${PGHOST#\[}"; export PGHOST="${PGHOST%\]}"
export PGUSER=$(jq -r --arg id "$container_id" \
  'select(.type == "container" and .container_id == $id) | .username' "$registry")
export PGPASSWORD=$(jq -r --arg id "$container_id" \
  'select(.type == "container" and .container_id == $id) | .password' "$registry")
PGSSLMODE=disable psql
unset PGPASSWORD PGUSER PGDATABASE PGHOST PGPORT
```

Only failed tests' private PostgreSQL databases are preserved; successful tests'
databases are still dropped. In keep mode, this decision happens in `TestMain`
after `m.Run`, when all per-test cleanups have finished, so late cleanup failures
also preserve their test's data. Without keep mode, databases are dropped during
their test's cleanup as usual. Shared template/maintenance databases can remain
inside a retained server. A package failure without a failed database fixture
can retain the container without retaining any private test database.

## Inspect SpiceDB from the host

Install the **Authzed `zed` CLI**, which is a SpiceDB client, not the Zed editor.
Connect to the recorded host-mapped endpoint, not a container-network alias, and
use the **exact fixture token**. In `serve-testing`, the token selects the datastore;
a different token selects a different, empty datastore even on the same server.
The shared relations helper and legacy helper have different schemas, so inspect
the schema first. The upstream [headless usage documentation](https://github.com/authzed/zed/blob/5c4c322e0507f90a080c426c0312459eb9443ece/README.md#headless-usage)
confirms `ZED_ENDPOINT`, `ZED_TOKEN`, and `zed schema read`; the
[command definitions](https://github.com/authzed/zed/blob/5c4c322e0507f90a080c426c0312459eb9443ece/internal/cmd/cmd.go)
define `--insecure` as a plaintext connection:

```sh
set +x
registry=/tmp/inventory-container-debug/registry.data.test.12345.jsonl
test_name=TestCreateRelationship
container_id=$(jq -r --arg test "$test_name" \
  'select(.type == "fixture" and .kind == "spicedb" and .test == $test) | .container_id' "$registry" | tail -n 1)
export ZED_TOKEN=$(jq -r --arg test "$test_name" \
  'select(.type == "fixture" and .kind == "spicedb" and .test == $test) | .token' "$registry" | tail -n 1)
export ZED_ENDPOINT=$(jq -r --arg id "$container_id" \
  'select(.type == "container" and .container_id == $id) | .endpoint' "$registry")
zed --insecure schema read
# Use schema-valid types/relations for relationship reads or permission checks.
unset ZED_TOKEN ZED_ENDPOINT
```

These local testing endpoints use plaintext gRPC; do not use `--insecure` for
production. With stdout-only diagnostics, no token is disclosed, so use the
credentials-file option when you need to reconnect. SpiceDB's retained in-memory
server keeps all its fixture datastores until the container stops; unlike private
PostgreSQL databases, successful SpiceDB datastores are not individually removed.

## Finish debugging

For each retained ID reported in the output:

```sh
# container_id must be the exact retained ID you want to remove.
podman rm -f -v "$container_id"
```

The `-v` flag removes anonymous volumes too, including PostgreSQL's data volume.
Removing a SpiceDB container destroys its in-memory datastores. Remove the exact
credential registry files manually when finished; container cleanup does not delete
them. Avoid broad cleanup commands that could delete unrelated development
containers or credentials.
