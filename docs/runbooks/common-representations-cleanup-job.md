# Common representations cleanup job

`run-job common-representations-cleanup-job` prunes common representation
history on resources with at least one active reporter. It requires PostgreSQL
and defaults to a read-only preview. The implementation and focused tests are
kept together in `cmd/jobs` for removal after this one-time operation; no cleanup
interfaces or application services are added under `internal/`. Tombstones,
orphans, reporter history, and
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

Each chunk freezes the oldest retained version per resource in memory. After the
scan transaction commits, the job records a WAL insertion position and waits for
the configured Debezium slot's durable acknowledgment to reach it. Only then does
it sample the tuple topic's end offset and wait for Inventory's committed restart
offset to reach that fixed target. Writes may continue throughout; the targets
never advance during the wait.

Every deletion follows one path: find the live retention threshold, clamp it to
the frozen cutoff, and delete at most `--batch-size` rows below both. Versions
written after capture survive this pass even when that leaves more than
`--keep-versions` rows. Each batch rechecks source progress, topic identity,
consumer offsets, active reporters and pointer consistency. SQL enforces the row
limit. Batches use the existing serializable transaction manager, with 30-second
statement and 5-second lock timeouts. The invocation deadline bounds startup,
boundary waits, transactions, and delays; no database transaction stays open
while waiting on Kafka or CDC.

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
  --keep-versions 2 --chunk-rows 20000 --replication-slot inventory_api_debezium \
  --batch-size 1000 --max-batches 100000 --batch-delay 500ms \
  --max-duration 2h --execute
```

After interruption, rerun with the same retention and scope plus
`--start-after <last_cursor>`. Cancellation does not undo committed batches.
Investigate inconsistent pointers rather than weakening their exclusion.

## Consumer history and service traffic

Reads and writes remain enabled. Execution requires `--replication-slot` and the
existing `consumer` broker/authentication/TLS, topic, and group configuration.
The job uses read-only Kafka admin operations; it never joins the consumer group,
produces marker events, changes offsets, or pauses the consumer. Preview does not
require Kafka or a replication slot.

Before execution, verify all of these deployment assumptions:

- The named slot belongs to the Inventory Debezium connector on the same primary
  database, uses `pgoutput`, and acknowledges WAL only after source offsets have
  been durably committed. This is the Debezium commit callback behavior reviewed
  for this design; verify the deployed connector/Connect versions and settings.
- The connector routes every relevant transactional `kessel.tuples` message to
  the configured topic. Do not use error-tolerant dropping of these events.
- Inventory's actual consumer group uses explicit offset commits, has relations
  replication enabled, and processes the single-partition tuple topic. Missing
  group offsets, a retention gap, or a different topic shape stop execution.
- The Job has read access to `pg_replication_slots` and Kafka topic/group metadata,
  in addition to its existing database cleanup privileges.

The broker-reported committed offset must reach the sampled *end* offset. This
also protects against replay of the last message in an uncommitted consumer
batch. With sparse traffic, offset batching and Debezium heartbeat/flush intervals
can extend the wait; expiry returns an error before any unverified deletion.

Do not reset connector/group offsets, recreate the slot/topic, fail over the
database, or rewind events during cleanup. The job detects topic identity and
partition changes, database restart, source checkpoint regression, and consumer
progress falling behind its target when checked before a batch. These checks do
not make concurrent administrative resets atomic with database deletion. Stop the
job before such changes; a rerun captures and verifies fresh cutoffs.

A missing predecessor can cause obsolete authorization to survive consumer
processing. The fixed cutoffs and consumption gate prevent this job from pruning
history still needed by the normal pipeline under the verified assumptions above.
They do not reconcile existing stale tuples or support arbitrary historical
replays after pruning. Establish the restore/rebuild procedure before rewinding.
Pruning also removes historical transaction IDs used for client retry deduplication;
the acceptable idempotency history window remains a separate policy decision.

Deletion creates dead tuples; allow autovacuum and autoanalyze to work before
measuring the effect. Physical compaction is a separate follow-up if measurements
show it is needed. Validate query plans and batch cost at representative scale;
local and ephemeral tests do not establish production readiness.

The earlier [PR #1508 performance experiment](https://docs.google.com/document/d/18xugoI-Tv_YEsegl0QufNQyRw7xrChZbVW0-gqK0rEY/edit?tab=t.pwyzu9l7nrdf)
used age-based retention across a broader resource scope, 10,000-row batches,
and separate read measurements before/after deletion and compaction. Its cleanup
runtime and total table shrink do not directly predict this job's results.

## OpenShift

The one-time Job template uses the configured Inventory image and Inventory YAML
Secret. It mounts the Clowder application Secret (`CLOWDER_CONFIG_SECRET_NAME`,
default `kessel-inventory`) at `/cdapp` and sets `ACG_CONFIG`. The existing
Clowder loader supplies PostgreSQL credentials and enables `verify-full` with
the supplied RDS CA when present. Kafka brokers and SASL credentials come from
that same Clowder configuration; the topic and consumer group come from the
Inventory YAML and application defaults. The optional `CA_CERT_SECRET`
(default `kessel-kafka-connect-config`) is mounted at `/kafka-ca-certs`, matching
the API deployment for configurations that specify a Kafka CA file. The former
`CLOWDER_ENABLED` parameter has
been removed because app-common-go does not read it. `EXECUTE` defaults to false.
The Job reuses the existing Inventory API service account (`SERVICE_ACCOUNT_NAME`,
default `kessel-inventory-api`) for its image pull credentials, with API token
mounting disabled. It creates no service account or RBAC resources.

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
`START_AFTER` to the final report's `last_cursor` and verify `REPLICATION_SLOT`
(default `inventory_api_debezium`). Set `ACTIVE_DEADLINE_SECONDS`
above `MAX_DURATION` to allow image pull and startup time. Review final logs and
Job status, including failures, before removing the completed Job object.

Ephemeral deployments use a ConfigMap for the Inventory YAML. For a test run,
copy that YAML into a uniquely named temporary Secret and set
`INVENTORY_API_CONFIG_NAME` to it; remove the test Secret afterward. Do not
change the deployment's configuration source.

### Stage deployment

The stage deployment definitions use `kessel-stage` on **hccs01ue1** for the
active Inventory writer and consumer. Run the Job in that namespace on that
cluster so its `inventory-api-config` and `kessel-inventory` Secrets resolve
locally. The older **crcs02ue1** stage deployment shares the database but has its
consumer disabled; its configuration will be rejected for cleanup execution.
Use an explicit OpenShift context for both processing and applying the template.

The Debezium `kessel-inventory-api-connector` runs under `kessel-kafka-connect`
in the separate `kessel-kafka-connect-stage` namespace on hccs01ue1. The Job
needs no Connect service URL or Kubernetes access to that namespace: it reads
`inventory_api_debezium` on the Inventory primary and queries Kafka directly.
The connector template uses `pgoutput`, routes `kessel.tuples` logical messages
through the outbox transform, and defaults to a five-minute heartbeat. The
Inventory topic is `outbox.event.kessel.tuples` with one partition, and the
default consumer group is `inventory-consumer` with explicit commits every
10 messages.

These mappings come from the [stage namespace configuration](https://gitlab.cee.redhat.com/service/app-interface/-/blob/master/data/services/insights/kessel/namespaces/hccs01ue1-kessel.yml),
[Connect deployment targets](https://gitlab.cee.redhat.com/service/app-interface/-/blob/master/data/services/insights/kessel/deploy-kessel-kafka-connect.yml),
and [Inventory connector template](https://github.com/project-kessel/kessel-kafka-connect/blob/main/deploy/inventory-api-connector.yml).

Allow more than the five-minute default invocation budget for a stage execution
(for example, `MAX_DURATION=30m`, `ACTIVE_DEADLINE_SECONDS=2100`); heartbeat and
Connect offset flushing can otherwise exhaust the wait. Sparse tuple traffic
can also delay the consumer's committed restart position. A timeout deletes no
unverified chunk; it does not justify weakening the gate.

Before the first stage execution, read the live slot, topic partition count,
group offsets, and connector status/settings using the deployed credentials.
Template inspection cannot establish their live state or Kafka metadata ACLs.
An ordinary preview only scans PostgreSQL and does not exercise the consumption
gate.

## Verification

```sh
CONTAINER_RUNTIME=podman ./scripts/testcontainers-run.sh \
  go test -race -tags=integration ./cmd/jobs \
  -run '^TestCommonRepCleanup' -count=1 -timeout 15m
```

The focused PostgreSQL suite covers retention and gaps, NULL and inconsistent
pointers, tombstoned and shared roots, resource type filtering, deep histories,
hard batch limits, interrupted chunk resume, frozen cutoffs under later writes,
boundary failure before and after committed work, read-only enforcement, and rollback.
