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

Pruning may remove the last remaining copy of a transaction ID used for
duplicate detection; a later report retry then follows normal validation and
processing, consistent with the agreed implementation policy.

The job does **not** query Kafka/CDC state, establish consumer drain, or fence
an application operation already in flight. Update consumption reads event
version v and v-1, so pruning must not run while pending update or delete
events reference versions inside the prunable window.
`--acknowledge-drained-consumer` is a manual attestation gate only; it is not a
safety check. Before any destructive run, an authorized operator must use the
environment's independent operational evidence to verify that applicable
update and delete processing is completed and committed, and that no relevant
application operation is in flight at the cleanup boundary. Replay or offset
rewind that requires pruned history is outside the supported contract;
restore a consistent database state before any offset rewind.

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

Execution requires both flags below, in addition to a fresh reviewed preview
and the operational preconditions above:

```sh
inventory-api run-job common-representations-cleanup-job \
  --keep-versions 2 \
  --batch-size 1000 \
  --max-batches 1 \
  --batch-delay 500ms \
  --max-duration 5m \
  --execute \
  --acknowledge-drained-consumer
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

## Stop conditions and reconciliation

Do not execute, or stop before another batch, if the target/scope is
uncertain, update or delete processing or application quiescence is
unverified, a new inconsistent-pointer exclusion appears, the report does not
match the reviewed bounds, or required observation or recovery evidence is
unavailable. Never override an exclusion by weakening the retention rule or
the pointer consistency check.

No backup/recovery procedure, numerical stop limits, or staffed
stage-validation plan is approved by this document. Obtain those approvals and
complete staffed stage validation before considering production; do not infer
them from the sample defaults or invent them during a run.

The job needs only its normal PostgreSQL connection and standard DML
privileges: it uses CTEs within the existing transaction manager, not
temporary tables, and it creates no indexes or other DDL. No additional index
is required by this job's selectors. Deleting rows reduces the number of
versions each consumer lookup must walk, which reduces query plan cost; it is
not by itself a physical disk reclamation or latency claim. Autovacuum is
expected to reclaim dead tuples; compaction (pg_repack or VACUUM FULL) is a
separate, conditional follow-up if measured improvements do not materialize.

After a run, reconcile committed common representation rows, distinct impacted
roots, retained versions per resource, exclusion counts, and service/database
health against the fresh preview. Investigate any difference before rerunning.
This job does not establish stage readiness, production policy, or a
replay/recovery horizon; those require separate validation and approval.

## Local verification

The implementation has passed the repository's `make test` race/short suite
and race-enabled PostgreSQL integration tests covering per-resource retention
boundaries, NULL-pointer fallback, inconsistent-pointer exclusion,
out-of-scope tombstoned-only roots, shared roots, resource type filtering,
most-versions-first batch targeting, bounded batch accounting with rerun,
read-only enforcement, and API revival
with consumer v/v-1 reads against pruned state. They do not establish the
operator prerequisites, stage readiness, or production approval.
