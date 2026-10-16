# Common representations cleanup job

`run-job common-representations-cleanup-job` prunes common representation
history on resources with at least one active reporter. It requires PostgreSQL
and defaults to a read-only preview. Tombstones, orphans, reporter history, and
compaction are separate workstreams.

## Retention

`--keep-versions` keeps the newest existing rows at or below each resource's
`common_version` pointer (default 2, minimum 2). A NULL pointer falls back to
that resource's maximum version. A pointer that does not resolve excludes the
whole resource. Versions above a non-NULL pointer are never deleted. For
example, versions `{0, 2, 4}` with current version 4 and keep=2 retain `{2, 4}`.
Retention is per resource, independent of row age or the job's start time.

Resource roots, reporter rows, and retained common representations survive.
Both preview and deletion exclude roots without an active reporter. Deletion
rechecks reporter state, pointer consistency, and the retention threshold in
its transaction, including after serialization retries.

## Batching and resume

The job scans the existing `(resource_id, version)` primary key in resource ID
order. `--chunk-rows` is a target, not a hard scan cap: the boundary resource's
whole history is included. A resource with very deep history can exceed the
target and the scan can hit the statement timeout. This avoids a table-wide
ranking before every batch; it does **not** implement global most-versions-first
priority. Within each resource, deletion selects the lowest versions first.

Every deletion follows one path: find each resource's oldest retained version
through the primary key, select older rows, and delete at most `--batch-size`
rows. SQL enforces this limit even when reports add versions between the scan
and deletion. Batches use the existing serializable transaction manager, with
30-second statement and 5-second lock timeouts. The invocation context also
bounds database startup, work, and delays.

`--max-batches` limits committed nonempty batches; `--max-duration` bounds total
runtime. Defaults are one batch of at most 1,000 rows, a 500ms delay, a 20,000-row
scan target, and five minutes. SIGINT/SIGTERM, deadline expiry, or a database
error stop the run and preserve earlier commits. Exhausted serialization
retries return an error; failed resources are not silently skipped.

The final report's `last_cursor` is the last fully processed chunk boundary.
Resume with `--start-after <last_cursor>`. If a chunk was interrupted, it is
scanned again so no unfinished resource is skipped. `pass_complete` means the
scan reached the end; concurrent writes behind the cursor require another
pass. When an exact full batch consumes the budget, completion may require a
rerun to check for more rows. A completed pass over unchanged data is idempotent.

Preview and execution share the same chunk scan. Counts describe that scan
window, not one database snapshot, and execution can differ from preview under
concurrent writes. Final deletion totals count committed rows and distinct
roots only. On cancellation or error, both modes log partial scan progress;
the command returns a nonzero exit status.

## Preview and execute

Use the normal Inventory executable and PostgreSQL configuration:

```sh
inventory-api run-job common-representations-cleanup-job \
  --keep-versions 2 --chunk-rows 20000 --max-duration 30m
```

Add `--resource-type <type>` to restrict the resource type. Review the scope,
prunable row counts, and inconsistent-pointer exclusions before execution:

```sh
inventory-api run-job common-representations-cleanup-job \
  --keep-versions 2 --chunk-rows 20000 \
  --batch-size 1000 --max-batches 100000 --batch-delay 500ms \
  --max-duration 2h --execute
```

After interruption, rerun with the same retention and scope plus
`--start-after <last_cursor>`. Cancellation does not undo committed batches.
Investigate inconsistent pointers rather than weakening their exclusion.

## Consumer history and service traffic

Current API reads and writes continue using retained versions. Pending consumer
events can reference older versions that this job removes: a missing previous
version produces a create-only tuple delta; a missing current version becomes
a no-op. If a workspace changed across that boundary, the old workspace tuple
can remain in the relations store. This can retain authorization and there is
no automatic reconciliation in this job.

Check consumer lag and service/database health before and during execution.
Keeping two versions does not guarantee safety for a delayed consumer or an
offset rewind. Pruning also removes historical transaction IDs used for retry
deduplication. Establish the acceptable history window and recovery procedure
before a production run; this runbook does not approve that run.

Deletion creates dead tuples; allow autovacuum and autoanalyze to work before
measuring the effect. Physical compaction is a separate follow-up if measurements
show it is needed. Validate query plans and batch cost at representative scale;
local and ephemeral tests do not establish production readiness.

## OpenShift

The one-time Job template uses the configured Inventory image, config secret,
and `kessel-inventory-db` credentials. `EXECUTE` defaults to false.

```sh
oc process -f deploy/kessel-inventory-common-representations-cleanup-job.yaml \
  -p JOB_NAME=common-rep-preview-<unique-suffix> \
  -p IMAGE_TAG=<immutable-tag> \
  -p KEEP_VERSIONS=2 -p MAX_DURATION=60m \
  -p ACTIVE_DEADLINE_SECONDS=3900 \
  | oc apply -f - -n <namespace>
oc logs -f job/common-rep-preview-<unique-suffix> -n <namespace>
```

For execution, use a new Job name and add `-p EXECUTE=true`, the reviewed
`BATCH_SIZE`, `MAX_BATCHES`, `BATCH_DELAY`, and `MAX_DURATION`. For resume, set
`START_AFTER` to the final report's `last_cursor`. Set `ACTIVE_DEADLINE_SECONDS`
above `MAX_DURATION` to allow image pull and startup time. Review final logs and
Job status, including failures, before removing the completed Job object.

## Verification

```sh
CONTAINER_RUNTIME=podman ./scripts/testcontainers-run.sh \
  go test -race -tags=integration ./cmd/jobs \
  -run '^TestCommonRepCleanupPostgres$' -count=1 -timeout 15m
```

The PostgreSQL suite covers retention and gaps, NULL and inconsistent pointers,
tombstoned and shared roots, resource type filtering, version over age, deep
histories, a hard batch row limit across roots, interrupted chunk resume,
idempotency, read-only enforcement, rollback, committed progress after failure,
serialization retries and exhaustion, pointer changes and concurrent reports,
post-prune revival and consumer v/v-1 reads, and randomized histories compared
with a reference retention model.
