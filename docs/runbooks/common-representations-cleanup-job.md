# Common representations cleanup job

> **Operational caution:** this job is an implementation of the common
> representation history cleanup path, not approval to run it against stage or
> production. No destructive run is authorized by this document.

## Scope and safety contract

The command is `run-job common-representations-cleanup-job`. It is
PostgreSQL-only and defaults to a read-only preview. It prunes
`common_representations` history on resources that still have at least one
active (non-tombstoned) reporter, keeping the newest `--keep-versions` common
representations per resource (default 2). The retention rule is per resource:
it is never anchored to the job run date.

The authoritative current version of each resource is the
`resource.common_version` pointer; when the pointer is NULL the job falls back
to `MAX(version)` over that resource's common history. Roots whose pointer
does not resolve to an existing common representation are excluded and
reported as inconsistent, never guessed. Roots with only tombstoned reporters
are out of scope and untouched: they belong to the tombstone cleanup
workstream. Resource roots, reporter rows, and the retained newest versions
are never deleted. There is no global orphan sweep.

Retention counts existing rows, not numeric version slots: history with gaps
(for example versions 0, 2, and 4) retains the newest existing rows at or
below the authoritative pointer, and a resource with fewer existing rows than
the retention window is pruned entirely of nothing. Preserving existing
predecessors never reconstructs a numerically missing version, so consumer
v/v-1 reads still depend on that version existing when it was reported.
Common representations above a non-NULL pointer are never ranked or deleted.

Pruning may remove the last remaining copy of a transaction ID used for
duplicate detection; a later report retry then follows normal validation and
processing, consistent with the agreed implementation policy.

Concurrent operation: the job is designed to run while the Inventory API
serves reads and writes and while the tuple consumer is processing events.
It never deletes resource roots, reporter rows, or the retained newest
versions, so API lookups, report writes, and delete-event processing are
unaffected. Pending consumer events whose referenced versions were pruned
degrade rather than fail: an update event whose v-1 row was pruned produces a
create-only tuple delta, and an event whose own version was pruned is a no-op
whose offset is committed. This has been verified against the real consumer
code path with real PostgreSQL.

The residual risk of that degradation is **stale workspace tuple residue**:
when the consumer is behind and a resource's workspace changed across the
pruned boundary, the delete side of the tuple delta is lost and the old
workspace tuple remains in the relations store. This is authorization
residue, not data corruption, and it is bounded by consumer health. Before
any large destructive run, verify the consumer group is keeping up with the
outbox topic so residue stays negligible; there is no automated cleanup for
leaked tuples. Replay or offset rewind that requires pruned history is
outside the supported contract; restore a consistent database state before
any offset rewind.

## Preview

Use the normal Inventory API executable and approved PostgreSQL configuration:

```sh
inventory-api run-job common-representations-cleanup-job \
  --keep-versions 2 \
  --batch-size 1000 \
  --max-batches 1 \
  --batch-delay 500ms \
  --max-duration 5m
```

Without `--execute`, this performs a read-only PostgreSQL snapshot report and
makes no row or event mutations. Add `--resource-type <type>` only when an
explicit resource scope is intended. Record the evaluation time, database
snapshot time, retention rule, scope, and report before considering any
execution.

The report distinguishes scoped active roots, roots with common history but no
active reporter (out of scope), roots excluded for an unresolvable
`common_version` pointer, total prunable rows and roots, and the bounded
projection for the configured row budget. Standard logs contain counts and
timestamps, not resource IDs or versions.

The configured row upper bound is `--batch-size * --max-batches`. Batches select
resources with the most versions first, ordering by the authoritative current
version descending, and process the oldest versions within each resource; ties
are broken deterministically by resource id. Once a resource is pruned to the
retention window it leaves the prunable set, so later batches naturally target
the next-highest resources. The
five-minute default runtime includes context-aware PostgreSQL startup,
preview, batch work, and delays; it can stop execution earlier, so a preview
of the row budget is not a promise that every bounded row will be processed.
Conservative defaults are 1,000 rows per batch, one batch, 500ms between
committed batches, and five minutes total runtime. Adjust bounds only under
the applicable operator approval. SIGINT and SIGTERM cancel startup or future
cleanup work; cancellation does not undo already committed batches.

## Destructive execution gate

Execution requires the flag below, in addition to a fresh reviewed preview and
the consumer-health check described above:

```sh
inventory-api run-job common-representations-cleanup-job \
  --keep-versions 2 \
  --batch-size 1000 \
  --max-batches 1 \
  --batch-delay 500ms \
  --max-duration 5m \
  --execute
```

The command takes a read-only preview first, using the same retention rule and
filters, then reselects and rechecks eligibility inside each bounded
transaction. The candidate set may change under concurrency; there is no
immutable worklist or durable checkpoint. The command logs each successful
commit and the total counts from committed batches only.

Cancellation or an error stops future batches; it does not undo commits
already completed. After an interrupted or limited run, reconcile committed
counts and current state, obtain a fresh preview, and rerun only the
still-approved scope. Do not infer completion from exit status alone if
earlier batches committed. `batch_limit_reached` means the configured maximum
was reached; it is not proof that no candidates remain.

## Running in OpenShift

`deploy/kessel-inventory-common-representations-cleanup-job.yaml` renders the
job as a bounded one-time OpenShift Job using the deployed Inventory image and
its database secret. `EXECUTE` defaults to `false`, so a plain render is a
read-only preview.

Preview:

```sh
oc process -f deploy/kessel-inventory-common-representations-cleanup-job.yaml \
  -p JOB_NAME=common-rep-cleanup-preview-$(date -u +%Y%m%dT%H%M%SZ) \
  -p IMAGE_TAG=<immutable-image-tag> \
  -p KEEP_VERSIONS=2 \
  -p BATCH_SIZE=1000 -p MAX_BATCHES=10 -p MAX_DURATION=10m \
  -p ACTIVE_DEADLINE_SECONDS=900 \
  | oc apply -f - -n <namespace>
oc logs -f job/<job-name> -n <namespace>
```

Destructive execution requires setting the gate parameter explicitly, in
addition to a fresh reviewed preview and the consumer-health check:

```sh
oc process -f deploy/kessel-inventory-common-representations-cleanup-job.yaml \
  -p JOB_NAME=common-rep-cleanup-$(date -u +%Y%m%dT%H%M%SZ) \
  -p IMAGE_TAG=<immutable-image-tag> \
  -p KEEP_VERSIONS=2 \
  -p BATCH_SIZE=1000 -p MAX_BATCHES=100 -p BATCH_DELAY=500ms -p MAX_DURATION=10m \
  -p ACTIVE_DEADLINE_SECONDS=900 \
  -p EXECUTE=true \
  | oc apply -f - -n <namespace>
```

`ACTIVE_DEADLINE_SECONDS` is the pod deadline and must exceed `MAX_DURATION`
plus image pull and startup margin; the job's own `MAX_DURATION` bounds its
work and SIGTERM cancellation keeps already-committed batches intact. Pausing
is a bounded run or a SIGTERM; resuming is a fresh preview plus another
bounded run. The template mounts the Inventory config secret named by
`INVENTORY_API_CONFIG_NAME` and reads PostgreSQL credentials from the
`kessel-inventory-db` secret, mirroring the existing one-time job templates.
Delete the completed Job object after reviewing its logs.

## Stop conditions and reconciliation

Do not execute, or stop before another batch, if the target/scope is
uncertain, the consumer group is falling significantly behind the outbox
topic, a new inconsistent-pointer exclusion appears, the report does not
match the reviewed bounds, or required observation or recovery evidence is
unavailable. Never override an exclusion by weakening the retention rule or
the pointer consistency check.

No backup/recovery procedure, numerical stop limits, or staffed
stage-validation plan is approved by this document. Obtain those approvals and
complete staffed stage validation before considering production; do not infer
them from the sample defaults or invent them during a run.

The job needs only its normal PostgreSQL connection and standard DML
privileges: it uses CTEs within the existing transaction manager, not
temporary tables, and it creates no indexes or other DDL. The existing
primary key and reporter-resource indexes are sufficient for the selectors to
execute correctly; whether they are sufficient for acceptable batch duration
at production scale is a separate question that stage validation must
measure. Every destructive batch classifies all scoped roots twice (candidate
selection and deletion) inside one serializable transaction, and each
statement is bounded by a 30-second statement timeout (further bounded by the
remaining invocation time). If classification exceeds that timeout at scale,
the batch fails safely with no partial commit; stage validation should
capture `EXPLAIN (ANALYZE, BUFFERS)` on representative data before any
production run. Deleting rows reduces the number of versions each consumer
lookup must walk, which reduces query plan cost; it is not by itself a
physical disk reclamation or latency claim. Autovacuum is expected to reclaim
dead tuples; compaction (pg_repack or VACUUM FULL) is a separate, conditional
follow-up if measured improvements do not materialize.

After a run, reconcile committed common representation rows, distinct impacted
roots, retained versions per resource, exclusion counts, and service/database
health against the fresh preview. Investigate any difference before rerunning.
This job does not establish stage readiness, production policy, or a
replay/recovery horizon; those require separate validation and approval.

## Local verification

The implementation has passed the repository's `make test` race/short suite
and the race-enabled real-PostgreSQL integration suite, run with:

```sh
CONTAINER_RUNTIME=podman ./scripts/testcontainers-run.sh \
  go test -race -short -tags=integration ./cmd/jobs \
  -run '^TestCommonRepCleanupPostgres$' -count=1 -timeout 15m
```

Coverage includes per-resource retention boundaries, NULL-pointer fallback,
inconsistent-pointer exclusion, out-of-scope tombstoned-only roots, shared
roots, resource type filtering, gapped-history retention, version authority
over row age, deterministic resource-id tie-breaking, most-versions-first
batch targeting, bounded batch accounting with rerun, read-only enforcement,
rollback after injected mid-batch failures, committed-then-failed batch
progress accounting, injected serialization-failure retry with exactly-once
deletion, concurrent-write conflict retry against new state, post-prune
tombstone and revival with root identity and numbering continuation, and
consumer v/v-1 reads against pruned state. They do not establish the operator
prerequisites, stage readiness, or production approval.
