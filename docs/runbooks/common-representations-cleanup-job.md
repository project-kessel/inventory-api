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

## Pass model

The job walks `common_representations` in primary-key (`resource_id`) order
using the existing primary-key index. Each chunk scan classifies the next
`--chunk-rows` rows' worth of roots (chunks are root-aligned: a root's full
history always lands in one chunk, so the `MAX(version)` fallback and pointer
checks see complete histories). Deletions are committed in bounded
transactions: small roots are grouped until their planned rows reach
`--batch-size`, and a single root deeper than the batch size is pruned by
repeated bounded lowest-version statements. Every delete transaction
re-classifies its roots inside the transaction, so concurrent reports are
always pruned against current state, never against a stale plan.

Per-statement cost is bounded by the chunk or batch size, not by table size:
total scan work for a pass is one index walk of the table, which is what makes
production-scale passes feasible. A full pass at production scale is expected
to take tens of minutes warm and possibly hours cold; budget `--max-duration`
and `ACTIVE_DEADLINE_SECONDS` accordingly.

The pass is resumable: an interrupted run reports its `last_cursor` (the last
fully processed resource id) and log progress per chunk; rerun with
`--start-after <cursor>` to continue. A rerun without the cursor re-scans the
cleaned prefix, which is correct but wasteful. `pass_complete` means the scan
reached the end of the key space with nothing prunable left below the cursor
at scan time; rows inserted behind the cursor by concurrent writers are picked
up by the next pass. A second completed pass deletes nothing.

Roots whose deletions repeatedly hit serialization conflicts (for example
extremely hot resources being reported continuously) are parked: the pass
skips them, completes, and reports them by resource id. Parked roots are not
an error; rerun the job to retry them.

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
residue, not data corruption, and it is bounded by consumer health. A
standing consumer backlog is disqualifying for a destructive run: during
the observed incident class (roughly 200k pending events taking a day or
more to drain), events still in the queue reference versions a concurrent
pass would prune, so duplicate-retry detection for their transaction IDs
degrades to normal validation and the delete side of their tuple deltas is
lost. Before any destructive run, verify the consumer group is keeping up
with the outbox topic — drain time in minutes, not hours; there is no
automated cleanup for leaked tuples. Replay or offset rewind that requires
pruned history is outside the supported contract; restore a consistent
database state before any offset rewind.

## Preview

Use the normal Inventory API executable and approved PostgreSQL configuration:

```sh
inventory-api run-job common-representations-cleanup-job \
  --keep-versions 2 \
  --chunk-rows 20000 \
  --max-duration 30m
```

Without `--execute`, this performs a read-only chunked scan and makes no row
or event mutations. Add `--resource-type <type>` only when an explicit
resource scope is intended. The scan reports progress per chunk and stops
with partial results (reported, not failed) if `--max-duration` is exhausted
before the pass completes; widen the duration for a full count. Because the
scan is chunked, totals describe the scan window, not a single database
snapshot. Record the scan times, retention rule, scope, and report before
considering any execution.

The report distinguishes scanned roots, roots with common history but no
active reporter (out of scope), roots excluded for an unresolvable
`common_version` pointer, and prunable rows and roots. Standard logs contain
counts, timestamps, and chunk cursors, not resource IDs or versions (parked
roots are the deliberate exception). Conservative defaults are 20,000 rows
per chunk, five minutes total runtime, and one batch; execution defaults are
deliberately tiny for a first cautious run. SIGINT and SIGTERM cancel startup
or future cleanup work; cancellation does not undo already committed batches.

## Destructive execution gate

Execution requires the flag below, in addition to a fresh reviewed preview and
the consumer-health check described above:

```sh
inventory-api run-job common-representations-cleanup-job \
  --keep-versions 2 \
  --chunk-rows 20000 \
  --batch-size 1000 \
  --max-batches 100000 \
  --batch-delay 500ms \
  --max-duration 2h \
  --execute
```

An executing run performs the same chunked scan and commits bounded
deletions as it goes; it does not take a separate preview first, so budget
`--max-duration` for one full pass. Every deletion reselects and rechecks
eligibility inside its bounded transaction. The command logs each chunk
scanned, each successful commit, and total counts from committed batches
only.

Cancellation or an error stops future batches; it does not undo commits
already completed. After an interrupted or limited run, note `last_cursor`
from the report and resume with `--start-after <cursor>` after re-verifying
the preconditions. `batch_limit_reached` means the configured maximum was
reached; it is not proof that no candidates remain. Parked roots are retried
by rerunning the pass.

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
  -p CHUNK_ROWS=20000 -p MAX_DURATION=60m \
  -p ACTIVE_DEADLINE_SECONDS=3900 \
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
  -p CHUNK_ROWS=20000 -p BATCH_SIZE=1000 -p MAX_BATCHES=100000 -p BATCH_DELAY=500ms \
  -p MAX_DURATION=2h -p START_AFTER=<last-cursor-or-00000000-0000-0000-0000-000000000000> \
  -p ACTIVE_DEADLINE_SECONDS=7800 \
  -p EXECUTE=true \
  | oc apply -f - -n <namespace>
```

`ACTIVE_DEADLINE_SECONDS` is the pod deadline and must exceed `MAX_DURATION`
plus image pull and startup margin; the job's own `MAX_DURATION` bounds its
work and SIGTERM cancellation keeps already-committed batches intact. Pausing
is a bounded run or a SIGTERM; resuming is `START_AFTER` set to the previous
run's `last_cursor`. The template mounts the Inventory config secret named by
`INVENTORY_API_CONFIG_NAME` and reads PostgreSQL credentials from the
`kessel-inventory-db` secret, mirroring the existing one-time job templates.
Delete the completed Job object after reviewing its logs.

## Stop conditions and reconciliation

Do not execute, or stop before another batch, if the target/scope is
uncertain, the consumer group has a standing backlog on the outbox topic
(the observed incident class of roughly 200k events taking a day or more to
drain is disqualifying), a new inconsistent-pointer exclusion appears, the
report does not match the reviewed bounds, or required observation or
recovery evidence is unavailable. Never override an exclusion by weakening
the retention rule or the pointer consistency check.

No backup/recovery procedure, numerical stop limits, or staffed
stage-validation plan is approved by this document. Obtain those approvals and
complete staffed stage validation before considering production; do not infer
them from the sample defaults or invent them during a run.

The job needs only its normal PostgreSQL connection and standard DML
privileges: it uses CTEs within the existing transaction manager, not
temporary tables, and it creates no indexes or other DDL. The existing
primary key index carries the chunk scan and the per-root deletes, so no
statement's cost grows with table size. Every statement is bounded by a
30-second statement timeout (further bounded by the remaining invocation
time); a statement that exceeds it fails safely with no partial commit. The
job's own deletes generate dead tuples that autovacuum reclaims behind the
pass because transactions are short; expect dead-tuple churn and reusable
space rather than on-disk shrink. Deleting rows reduces the number of
versions each consumer lookup must walk, which reduces query plan cost; it is
not by itself a physical disk reclamation or latency claim. Compaction
(pg_repack or VACUUM FULL) is a separate, conditional follow-up if measured
improvements do not materialize.

After a run, reconcile committed common representation rows, distinct impacted
roots, retained versions per resource, exclusion counts, parked roots, and
service/database health against the fresh preview. Investigate any difference
before rerunning. This job does not establish stage readiness, production
policy, or a replay/recovery horizon; those require separate validation and
approval.

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
over row age, root-aligned chunk boundaries with deep roots pruned in bounded
batches, cursor resume completing a partial pass, pass idempotency, bounded
batch accounting, read-only enforcement, rollback after injected mid-batch
failures, committed-then-failed batch progress accounting, injected
serialization-failure retry with exactly-once deletion, parking of
serialization-exhausted hot roots while the pass continues, concurrent-write
re-classification against new state, post-prune tombstone and revival with
root identity, numbering continuation, and consumer v/v-1 reads against
pruned state, plus twenty randomized history scenarios verified against a
reference model of the retention rule. They do not establish the operator
prerequisites, stage readiness, or production approval.
