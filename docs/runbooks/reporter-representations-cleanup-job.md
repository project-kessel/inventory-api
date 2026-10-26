# Reporter representations cleanup job

This disposable PostgreSQL maintenance job follows the common cleanup structure.
Implementation and focused tests live in `cmd/jobs/reporterrepcleanup`; only
command registration lives in `cmd/jobs`. This is the same intentional placement
exception for one-time tooling. No migrations, persistent worklist, Kafka client,
shared framework, or new Kubernetes permissions are added.

## Retention and scope

Keep the newest **two existing rows per active reporter-resource**, regardless of
age. `--keep-versions` can increase this number but cannot be less than two.
There are no resource-type or reporter-type filters and no seven-day policy.

Join history to its own `reporter_resources` parent and require the parent to be
non-tombstoned. Rank by `(generation DESC, version DESC)` separately for each
`reporter_resource_id`, including historical tombstone rows. Generation matters:
recreation increments generation and resets version. Multiple reporters sharing
a resource retain their own history independently.

Only rows at or below the parent's `(generation, representation_version)` are
ranked; above-pointer rows are preserved. An exact pointer row need not exist:
common-only updates can advance the parent without creating reporter history.
Resources with fewer than the requested number keep everything. Currently
tombstoned parents and orphan history belong to separate cleanup work.

Pruning reduces historical transaction-ID deduplication coverage. An old report
whose ID has disappeared from both history tables can be processed again as a
new update to the existing resource. No age-based retry guarantee or arbitrary
historical replay support is introduced. This matches the accepted common
cleanup policy. The current internal consumer reads common history; these tests
do not establish safety for future reporter-based consumer behavior.

## Maintenance and batching

Preview is the default and counts candidates in a read-only transaction. It
creates no worklist and deletes nothing; normal traffic may continue for preview.
Counts can change while writers are active.

Execution requires `--execute --writers-paused`. These flags acknowledge, but do
not establish, the operator-verified [read-only and drain procedure](common-representations-cleanup-job.md#required-maintenance-window).
Use that procedure in stage and production: fence every resource write path,
wait for accepted writes to be published and successfully replicated, keep the
drained internal consumer running, and keep writers fenced through verification.
Do not put PostgreSQL itself into read-only mode. Stable row counts or low lag
alone do not demonstrate drain. If writers resume or the consumer restarts or
rebalances, stop cleanup and repeat drain verification before another invocation.

Execution populates one session-local temporary worklist of
`(reporter_resource_id, generation, version)`. A pinned connection owns it; it is
dropped on exit, including cancellation, and disappears on session loss. Each
transaction selects a bounded batch ordered by those keys, deletes history, and
removes the corresponding worklist entries. Count mismatches roll back the batch.
Only committed deletions are counted. No cursor or durable checkpoint is needed:
a rerun builds a fresh worklist from the remaining history.

Defaults: `--batch-size=1000 --batch-delay=500ms --max-batches=0`. Zero maximum
batches processes the entire worklist; a positive cap exits successfully with
`complete=false` when work remains. Cancellation or SQL failure returns an error;
earlier committed batches remain committed. Use a new Job name for every run.

There is no runtime limit or statement timeout, including inherited database/role
statement timeouts. The five-second lock-acquisition timeout remains. Cancellation
interrupts SQL and pacing. Measure selection and deletion before production use:
worklist selection can scan/sort substantial history and consume temporary disk.

Logs include `dry_run`, configuration, candidate count, selection duration,
projected batch count, committed batch durations/deletions, remaining rows, and
completion. `minimum_duration_seconds` counts only inter-batch pauses and excludes
SQL time. A completed preview means counting finished, not that rows were deleted.

## Apply the Job

The template uses the existing Inventory service account and YAML/Clowder Secrets
for PostgreSQL configuration, without mounting an API token. It runs no migrations.
Pin a compatible immutable image and use an explicit context and namespace.

```sh
oc --context=<context> -n <namespace> process --local \
  -f deploy/kessel-inventory-reporter-representations-cleanup-job.yaml \
  -p JOB_NAME=reporter-rep-preview-<unique-suffix> \
  -p IMAGE_TAG=<immutable-tag> \
  > /tmp/reporter-rep-preview.json
oc --context=<context> -n <namespace> apply -f /tmp/reporter-rep-preview.json
oc --context=<context> -n <namespace> logs -f job/reporter-rep-preview-<unique-suffix>
```

For deletion, verify maintenance preconditions, choose a **new Job name**, and add
`-p EXECUTE=true -p WRITERS_PAUSED=true`. Override `BATCH_SIZE`, `BATCH_DELAY`,
`MAX_BATCHES`, and `KEEP_VERSIONS` as reviewed. Check both Job status and logs;
a successful capped invocation is not necessarily complete.

For ephemeral tests use the reserved namespace explicitly (currently
`ephemeral-dl2fn7` on crc-eph; verify the live reservation). ConfigMap-based
Inventory deployments need their YAML copied to a uniquely named test Secret;
override `INVENTORY_API_CONFIG_NAME`, `CLOWDER_CONFIG_SECRET_NAME`, and image.
Do not reset shared Kafka topics or offsets. Verify exact retained keys, unchanged
parent/common rows, authorization, replay of the last processed event, and normal
report/update/delete/recreate after restoring writes.

## Local verification

```sh
go test -race ./cmd/jobs/... ./cmd -count=1
go test -race -tags=integration ./cmd/jobs/reporterrepcleanup -count=1 -v
```

The tagged suite uses an isolated real PostgreSQL database and covers independent
reporters, generation resets, missing pointer rows, above-pointer rows, timestamp
disorder, historical tombstones, currently tombstoned parents, keep-three,
preview, bounded batches, capped rerun, idempotency, cancellation, committed
progress after failure, atomic rollback, count mismatches, and a 100,000-row
fixture including one 50,000-row history. Default unit CI does not run this tagged
suite. Small synthetic timings do not predict production throughput.
