# Common representations cleanup job

This one-time, database-only job prunes history during a maintenance window.
It defaults to a read-only preview. All implementation and focused tests live in
`cmd/jobs/commonrepcleanup`; command registration remains in `cmd/jobs`. This is
an intentional exception for disposable maintenance tooling.
No permanent table, migration, Kafka client, or Kubernetes permission is added.

## Retention and scope

Default: `--keep-versions=2`. This is version-count retention, with no age window.

For each eligible resource, keep its authoritative current common representation
and its newest existing predecessors, ordered by version descending. Delete
only versions outside that retained set. `--keep-versions` is configurable but
must be at least two, because update consumption uses the current and previous
representations. Resources with fewer versions keep everything.

Age does not affect eligibility: recent, old, and nonmonotonic timestamps follow
the same rule. For a normal resource with ten versions, keeping two deletes eight
even if all ten were created today. `--retention-days` is not supported.

Only roots with an active, non-tombstoned reporter are in scope. Multiple active
reporters do not duplicate candidates. An optional `--resource-type` narrows the
scope. Tombstoned-only roots and orphaned representations belong to separate jobs.
A NULL `resource.common_version` falls back to the maximum existing version.
A non-NULL pointer that does not resolve to history excludes the root. Versions
above the authoritative pointer are never deleted; retention also preserves the
current pointer and its newest existing predecessors.

## Required maintenance window

`--execute --writers-paused` acknowledges these **operator-verified** preconditions.
The job does not enable read-only mode or check pipeline progress.

Use the same maintenance procedure in stage and production: put all resource
write paths into read-only mode, then let pending tuple replication finish before
cleanup. Read-only mode here blocks resource changes; PostgreSQL must remain
writable for consumer consistency-token updates and cleanup deletions.

1. Enable `--server.read-only=true` on every API/worker deployment that can accept
   resource writes. Wait for rollout and old writable pods/in-flight requests to
   finish. Fence alternate endpoints, backfill jobs, and direct resource writers.
   Do not set PostgreSQL itself read-only: the consumer still updates consistency
   tokens and the cleanup job must delete history. Reads may continue.
2. Leave Debezium and the Inventory consumer running to finish pending replication.
   Record the source checkpoint proving the last resource-changing transactions
   were published, then the resulting Kafka end position for each partition.
   Verify successful consumer processing through those positions, including
   authorization outcomes and absence of failed/skipped replication.
3. Idle pods, stable row counts, near-zero lag, and stable topic ends are not
   sufficient proof on their own. The current consumer commits raw message offsets
   and batches commits, so its committed restart position can lag successful
   processing on a quiet pipeline. Do not reset offsets or accept a fixed idle-time
   delay as a substitute for evidence. Source heartbeat/checkpoint liveness also
   needs verification on a quiet database. If final publication and processing
   cannot be demonstrated, do not execute cleanup.
4. Keep writers fenced throughout preview, deletion, and verification. The drained
   consumer remains running. If writers resume or the consumer restarts/rebalances,
   stop cleanup and repeat drain verification before another invocation.
5. Verify retained history and current/obsolete authorization after cleanup. Check
   consumer health and restart/replay behavior in stage before production use.
   Restore writers after verification; upstream retries may then create a backlog.

The maintenance interruption includes rollout, drain, cleanup, and verification.
Keeping the newest two versions is not a replacement for draining a backlog:
older queued events may require predecessors that pruning would remove.
Historical replay, offset resets, and rebuilding authorization from arbitrary old
messages remain outside the supported procedure. History pruning also reduces
historical transaction-ID deduplication coverage.

## Query and batching

Preview counts the candidate query in a read-only transaction, without temp-table
creation. Execution runs the same query once into a temporary worklist containing
only `(resource_id, version)`, with a primary key on those columns. One pinned
PostgreSQL connection owns the table throughout the invocation; it is explicitly
dropped on exit, including cancellation, and disappears with the database session.
There is no persistent progress state or cursor.

Each batch selects up to `--batch-size` entries ordered by `(resource_id, version)`
(oldest versions first within each resource). History deletion and worklist removal
commit together. A count mismatch aborts and rolls back the batch. Logged deletion
counts include only committed batches.

Defaults: 1,000 rows/batch, 500ms delay, and the entire worklist per invocation.
`--max-batches=0` processes until the worklist is empty; set a positive value for
a smaller trial run. Override `--batch-size` and `--batch-delay` as needed.
Cleanup transactions disable PostgreSQL statement timeouts, including inherited
database/role settings. There is no job runtime limit or pod execution deadline.
Stop the job manually when needed; cancellation interrupts active SQL and batch
pacing. The five-second lock-acquisition timeout remains. Worklist selection scans
eligible history once; measure selection and deletion before production execution.

At the batch limit the job exits successfully with `complete=false` and reports
remaining work. Cancellation or SQL failure returns an error; already committed
batches remain committed. Rerun with the same retention/scope and a new Job name:
it selects only remaining history. No `--start-after`, `--chunk-rows`, or
`--replication-slot` is accepted.

Logs report selection duration, candidate count, each committed batch's duration,
committed deletions, remaining worklist rows, and completion. A preview's remaining
count is its candidate count; `complete=true` means the preview finished, not that
history was deleted. Worklists are not durable artifacts after the invocation.

Preview logs explicitly announce that no history will be deleted, and all job
logs include `dry_run=true` for preview or `dry_run=false` for execution. Startup
logs show `batch_size`, `batch_delay`, and `max_batches`. After successful
selection, both modes report `projected_batches` and `minimum_duration_seconds`.
The projection respects `max_batches`; the minimum counts only pauses between
those batches, `(projected_batches - 1) * batch_delay`, or zero for no batches.
It excludes SQL/commit time and is not an estimate of the maintenance window.
Preview does not actually sleep or delete any rows.

## Apply the Job

The template uses existing Inventory YAML and Clowder Secrets for PostgreSQL
configuration, including RDS CA/`verify-full` when configured. It requires no Kafka
CA mount or Kafka access. The existing Inventory service account supplies image
pull credentials; its API token is not mounted. The Job runs no migrations.
A standalone approved cleanup image can be used without rolling out the serving
API image, provided its database-schema compatibility has been verified.

Use an explicit cluster context and namespace for every operation. For stage,
the active deployment is `kessel-stage` on **hccs01ue1**, using Secrets
`inventory-api-config` and `kessel-inventory`. The older **crcs02ue1** deployment
shares the database: account for all writers when establishing the fence.
The job no longer requires an enabled consumer in its own configuration.

```sh
oc --context=<stage-context> -n kessel-stage process --local \
  -f deploy/kessel-inventory-common-representations-cleanup-job.yaml \
  -p JOB_NAME=common-rep-preview-<unique-suffix> \
  -p IMAGE_TAG=<immutable-tag> \
  -p KEEP_VERSIONS=2 \
  > /tmp/common-rep-preview.json
oc --context=<stage-context> -n kessel-stage apply -f /tmp/common-rep-preview.json
oc --context=<stage-context> -n kessel-stage logs -f job/common-rep-preview-<unique-suffix>
```

For execution, verify the maintenance preconditions, choose a new Job name, and
add `-p EXECUTE=true -p WRITERS_PAUSED=true`. Set the reviewed `BATCH_SIZE`,
`MAX_BATCHES`, and `BATCH_DELAY`. Leave `MAX_BATCHES=0` to finish the whole
worklist, or set a positive value for a trial. Review logs and Job success/failure;
a successful but batch-limited Job does not imply cleanup is complete.

For ephemeral, target **ephemeral-kkotzy** explicitly. Deployments that use an
Inventory ConfigMap need its YAML copied to a uniquely named temporary Secret for
this template; override `INVENTORY_API_CONFIG_NAME`, `CLOWDER_CONFIG_SECRET_NAME`,
and image as appropriate. Use an isolated test database/pipeline; preserve the
walkthrough fixture and do not reset shared Kafka topics or consumer offsets.

### Read-only mode and pipeline drain (stage and production)

- Kessel Inventory Consumer (KIC) consumes `outbox.event.hbi.hosts` and
  `host-inventory.hbi.hosts` and writes resources through Inventory's API.
  Inventory's read-only mode must block writes from both streams, along with
  every other resource writer. Wait for old writable pods and in-flight writes
  to finish. The upstream HBI topics do not need to drain for cleanup once these
  writes are blocked; preserve pending events for processing after maintenance.
- Inventory's internal consumer processes `outbox.event.kessel.tuples` after
  resource writes stop. Leave Debezium and this consumer running until every
  resource change accepted before read-only mode has been published and
  successfully processed. Verify the source checkpoint and processing through
  every partition's resulting end position before cleanup. Leave this drained
  consumer running throughout cleanup.

In stage, the connector `kessel-inventory-api-connector` runs in
`kessel-kafka-connect-stage` on hccs01ue1, uses slot `inventory_api_debezium`, and
publishes `outbox.event.kessel.tuples`. The internal consumer defaults to group
`inventory-consumer`. Verify live connector/slot names, subscriptions, consumer
group IDs, and partition counts in the target environment; do not assume stage
names apply to production. Record the read-only coverage for all resource writers
and drain evidence for every configured downstream replication stream.

## Verification

```sh
go test ./cmd/jobs/... ./cmd -count=1
go test -race -tags=integration ./cmd/jobs/commonrepcleanup -run '^TestCommonRepCleanupPostgres$' -count=1 -v
```

The focused PostgreSQL suite covers version-count retention regardless of age or timestamp disorder,
NULL/inconsistent pointers, active reporter/type scope, bounded batches, rerun,
rollback of both history and worklist, cancellation, committed progress after
failure, worklist mismatch, and selection timing on a 100,000-row fixture including
a 50,000-version root. Tagged PostgreSQL tests do not run in default unit CI.
Small synthetic measurements are not stage/production throughput guarantees.
