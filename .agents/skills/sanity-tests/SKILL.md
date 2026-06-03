# Sanity Tests Skill

Run the E2E sanity test suite against an ephemeral OpenShift environment.

## When to Use

Use this skill when the user says any of:
- "run sanity tests"
- "run sanity tests against ephemeral"
- "run e2e sanity"
- "sanity check the ephemeral deployment"
- "test the ephemeral environment"
- "run sanity test in ephemeral"

## Prerequisites

- `oc` CLI must be logged in (`oc whoami` succeeds)
- VPN connected (required for bonfire to reach app-interface)
- Go toolchain available
- For `-b` mode: `podman` available and `QUAY_REPO` env var set
- For tests-only mode: an ephemeral namespace must already be deployed

## Steps

### 1. Verify OC login and VPN

```bash
oc whoami
```

If this fails with "Unauthorized", ask the user to log in with `oc login`.

### 2. Choose the run mode

Ask the user which mode unless they've already specified:

| Mode | Flag | When to use |
|------|------|-------------|
| **Tests only** | _(no flag)_ | Namespace already deployed with the code you want to test |
| **Build + deploy + test** | `-b` | Test the current branch from scratch (requires `QUAY_REPO`) |
| **Deploy + test** | `-d` | Deploy whatever is in bonfire config (no local build) |

### 3. Set QUAY_REPO (if using `-b` mode)

The user needs a Quay.io repo to push images to. Check if it's set:

```bash
echo "QUAY_REPO=${QUAY_REPO:-<not set>}"
```

If not set, ask the user for their Quay repo (e.g. `quay.io/<username>/kessel`).

### 4. Run the sanity tests

```bash
# Tests only (environment already deployed)
./scripts/run-sanity-tests.sh

# Build + push + deploy + test (full cycle)
QUAY_REPO=<repo> ./scripts/run-sanity-tests.sh -b

# Deploy existing bonfire config + test
./scripts/run-sanity-tests.sh -d

# Target a specific namespace
./scripts/run-sanity-tests.sh -n ephemeral-abc123
```

**IMPORTANT**: Run the script in the background with output monitoring:
- Set `block_until_ms: 0` so the agent can multitask
- Monitor with pattern `(Total:.*Passed:|FAIL|deploy failed|Deployed to)` to catch results
- The full cycle (`-b`) takes ~10-25 minutes; deploy-only (`-d`) takes ~5-15 minutes

Script flags:
- `-b` -- Build a fresh `linux/amd64` image, push to `QUAY_REPO`, update bonfire config, deploy, then test
- `-d` -- Deploy whatever is in bonfire config to ephemeral, then test
- `-n <ns>` -- Target a specific namespace (default: auto-detect via `oc project -q`)
- `-p <port>` -- Local API port-forward port (default: 9000)
- `-P <port>` -- Local DB port-forward port (default: 5432)

The script handles:
- Building and pushing the image (with `-b`)
- Updating `~/.config/bonfire/config.yaml` with the new image tag (with `-b`)
- Deploying via bonfire (with `-b` or `-d`)
- Discovering DB credentials from the `kessel-inventory-db` secret
- Setting up port-forwards to the API and DB
- Running `go test -v -count=1 ./test/e2e/sanity/ -timeout 10m`
- Cleaning up port-forwards on exit

### 5. Interpret results

- **PASS**: All tests passed. The branch is safe to merge.
- **FAIL**: Report which tests failed and the error messages. The most common failures:
  - Timeout polling for `ALLOWED_TRUE`: consumer may be slow or not running; check `kessel-inventory-consumer-*` pod logs
  - DB assertion mismatch: the API behavior has changed; compare expected vs actual values
  - Connection errors: port-forward may have dropped; restart and retry
  - `ReportDeleteReReport_Revive` timeout: known flake due to consumer latency; safe to re-run

### 6. Run specific test groups (optional)

```bash
# Only Check tests (Groups 1-3)
go test -v -count=1 -tags=sanity -run 'TestSanity_(Report|Delete|Check_)' ./test/e2e/sanity/ -timeout 10m

# Only CheckForUpdate tests (Group 4)
go test -v -count=1 -tags=sanity -run 'TestSanity_CheckForUpdate' ./test/e2e/sanity/ -timeout 10m

# Only Bulk tests (Group 5)
go test -v -count=1 -tags=sanity -run 'TestSanity_(CheckBulk|CheckSelf)' ./test/e2e/sanity/ -timeout 10m

# Only Lifecycle tests (Group 6)
go test -v -count=1 -tags=sanity -run 'TestSanity_(Revive|MultiResource)' ./test/e2e/sanity/ -timeout 10m

# Only Streaming tests (Group 7)
go test -v -count=1 -tags=sanity -run 'TestSanity_Streamed' ./test/e2e/sanity/ -timeout 10m
```

**Note**: The `-tags=sanity` flag is required. Without it, `go test ./...` skips these files
(they use `//go:build sanity` to avoid running in CI where no database is available).

## Test Coverage

| Group | File | RPCs Covered |
|-------|------|-------------|
| 1-3 | `check_test.go` | ReportResource, DeleteResource, Check |
| 4 | `checkforupdate_test.go` | CheckForUpdate, CheckForUpdateBulk |
| 5 | `bulk_test.go` | CheckBulk, CheckSelf, CheckSelfBulk |
| 6 | `lifecycle_test.go` | Report/Delete/Re-report lifecycle |
| 7 | `streaming_test.go` | StreamedListObjects, StreamedListSubjects |

## Troubleshooting

| Error | Cause | Fix |
|-------|-------|-----|
| `Unable to connect to app-interface` | VPN not connected | Connect to VPN, then retry |
| `Unauthorized` from `oc` | OC session expired | Run `oc login` |
| `QUAY_REPO not set` | Missing env var for `-b` mode | Set `QUAY_REPO=quay.io/<user>/kessel` |
| Port-forward errors | Port already in use | Use `-p`/`-P` flags for alternate ports |
| `PIDS[@]: unbound variable` | Old script version | Pull latest — this bug is fixed |
