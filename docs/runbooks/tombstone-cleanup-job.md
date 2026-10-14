# Tombstoned resources cleanup job

> **Operational caution:** this job is an implementation of the Phase 1 cleanup
> path, not approval to run it against stage or production. A seven-day threshold
> is a trial input, not a proven consumer/recovery horizon or production retention
> policy. No destructive run is authorized by this document.

## Scope and safety contract

The command is
`run-job tombstoned-resources-cleanup-job`. It is PostgreSQL-only and defaults to
a read-only preview. It selects `reporter_resources` rows where `tombstone IS
TRUE` and `updated_at` is **strictly older** than a frozen UTC cutoff. The caller
must pass a positive `--tombstone-days`; the command has no retention-age default.
The approved trial example is `--tombstone-days 7` (168 hours), and a row exactly
at the cutoff is retained.

For each selected reporter row, cleanup rechecks the age/filter predicate, root
type, natural-key shape, pointed terminal tombstone representation, and absence of
newer history inside the serializable batch transaction. Missing or inconsistent
state is excluded and reported, not repaired or guessed. Reporter history is
removed by the reporter-row cascade. Common history and a resource root are
removed only when the root was touched by the selected batch and no reporter row
survives. Shared roots and their common history remain. There is no global orphan
sweep. Transaction-ID duplicate detection may expire when valid tombstone cleanup
removes its last history copy; a later report may follow normal processing and
create a new resource.

The job does **not** query Kafka/CDC state, establish consumer drain, or fence an
application operation already in flight. `--acknowledge-drained-consumer` is a
manual attestation gate only; it is not a safety check. Before any destructive
run, an authorized operator must use the environment's independent operational
evidence to verify that applicable delete processing is completed and committed,
and that no relevant application operation is in flight at the cleanup boundary.
That evidence should include the applicable broker/CDC committed-offset snapshot
and the corresponding Relations-side delete outcomes, plus the environment's
evidence that no relevant application operation is in flight. If any part cannot
be demonstrated, do not execute. Serializable isolation, lag, elapsed time, or
this acknowledgement alone does not establish those conditions or prove
same-root replay safety.

### Known residual risk: no consumer read guard

This release does not include a consumer representation-read guard qualified by
resource root. Until such a guard ships, physical cleanup leaves two known
hazards if a delete event for a removed resource is pending, redelivered, or
replayed after the cleanup boundary:

- If the natural key was not reused, consumer delete processing cannot resolve
  its representations and the message fails processing, which can stall
  consumption for that partition until the message is resolved operationally.
- If the natural key was re-reported after cleanup (a new root with the same
  reporter key), the delete's natural-key lookup can resolve the live
  replacement and remove its Relations tuples. This is silent: it appears as a
  successfully processed delete.

The drained-consumer verification above and a no-offset-rewind discipline during
the retention window are therefore hard requirements, not best effort. Replaying
offsets into cleaned history is outside the supported contract; restore a
consistent database state before any offset rewind.

## Preview

Use the normal Inventory API executable and approved PostgreSQL configuration:

```sh
inventory-api run-job tombstoned-resources-cleanup-job \
  --tombstone-days 7 \
  --batch-size 1000 \
  --max-batches 1 \
  --batch-delay 500ms \
  --max-duration 5m
```

Without `--execute`, this performs a read-only PostgreSQL snapshot report and
makes no row or event mutations. Add `--reporter-type <type>` only when an
explicit reporter scope is intended. Record the evaluation time, database
snapshot time, cutoff, scope, and report before considering any execution.
In OpenShift, `deploy/kessel-inventory-tombstone-cleanup-job.yaml` renders a
bounded one-time Job that exposes these flags as template parameters; its
`EXECUTE` and `ACKNOWLEDGE_DRAINED_CONSUMER` parameters default to `false`.

The report distinguishes age candidates, tombstoned rows with an unknown (`NULL`)
`updated_at`, structurally eligible totals, bounded impact, and protected
exclusions. A NULL timestamp is not treated as old or eligible. It includes
reporter entities and metadata rows, reporter/common history rows, distinct
impacted roots, projected removable roots, and age-candidate exclusion counts by
reason. Standard logs contain counts and timestamps, not resource IDs or versions.
Preserve any separately collected exact identities only in approved controlled
artifacts.

### Controlled identity artifact

When exact candidate identities are needed for review, use
[`tombstone-cleanup-identities.sql`](tombstone-cleanup-identities.sql) with the
evaluation time, strict cutoff, normalized reporter filter (empty for unfiltered),
and bounded reporter-row limit from the same preview. There is no age or scope
default in this SQL file. It reproduces the job's strict age predicate, structural
reason precedence, deterministic `updated_at, reporter_resource_id` ordering, and
bounded eligible prefix. It also reports NULL `updated_at` tombstones separately
as `unknown_timestamp`; these rows are not age candidates.

The artifact opens its own `SERIALIZABLE READ ONLY` database snapshot. It is not
the same immutable snapshot as the application preview; concurrent changes can
alter results. Compare the identities and summary to a fresh preview and refresh
the preview if they differ. Do not infer that an older report and this artifact
describe an identical candidate set. It validates parameters without reading
Inventory data, then runs one `SELECT` in that read-only transaction; it creates
no tables and performs no Inventory/outbox mutation or index/DDL change.

Use the approved PostgreSQL service and a protected password file; do not put a
password or credential-bearing DSN in command arguments. Keep the service file
protected and free of passwords; keep the `PGPASSFILE` at mode `0600`. For
example, after setting the four values from the reviewed preview in the shell:

```sh
umask 077
identity_dir="/approved/private/path/tombstone-review-$(date -u +%Y%m%dT%H%M%SZ)"
mkdir -m 700 "$identity_dir"
PGSERVICEFILE="/approved/config/inventory.pg_service.conf" \
PGSERVICE="$APPROVED_PGSERVICE" \
PGPASSFILE="/approved/credentials/inventory.pgpass" \
psql -X -q --no-password --csv -P footer=off -v ON_ERROR_STOP=1 \
  -v evaluation_time_utc="$EVALUATION_TIME_UTC" \
  -v cutoff_utc="$STRICT_CUTOFF_UTC" \
  -v reporter_type_filter="$NORMALIZED_REPORTER_TYPE" \
  -v bounded_row_limit="$BOUNDED_REPORTER_ROW_LIMIT" \
  -f docs/runbooks/tombstone-cleanup-identities.sql \
  > "$identity_dir/tombstone-identities.csv"
chmod 600 "$identity_dir/tombstone-identities.csv"
```

Use an empty `NORMALIZED_REPORTER_TYPE` only when the reviewed preview was
unfiltered. The CSV contains exact reporter-resource and root UUIDs, pointer
version/generation, age, structural reason, and eligible-prefix position so
approval and post-run reconciliation use exact identity comparisons, not ID
prefixes. It deliberately omits representation payloads, consistency tokens
(`ktn`), transaction IDs, and credentials. Keep the directory and CSV in the
approved controlled location; do not paste them into ordinary logs, tickets, or
chat.

The configured reporter-row upper bound is `--batch-size * --max-batches`. The
five-minute default runtime includes context-aware PostgreSQL startup, preview,
batch work, and delays; it can stop execution earlier, so a preview of the row
budget is not a promise that every bounded row will be processed. Conservative
defaults are 1,000 rows per batch, one batch, 500ms between committed batches,
and five minutes total runtime. Adjust bounds only under the applicable operator
approval. SIGINT and SIGTERM cancel startup or future cleanup work; cancellation
does not undo already committed batches.

## Destructive execution gate

Execution requires both flags below, in addition to a fresh reviewed preview and
the operational preconditions above:

```sh
inventory-api run-job tombstoned-resources-cleanup-job \
  --tombstone-days 7 \
  --batch-size 1000 \
  --max-batches 1 \
  --batch-delay 500ms \
  --max-duration 5m \
  --execute \
  --acknowledge-drained-consumer
```

The command takes a read-only preview first, using the same frozen cutoff and
filters, then reselects and rechecks eligibility inside each bounded transaction.
The candidate set may change under concurrency; there is no immutable worklist or
durable checkpoint. The command logs each successful commit and the total counts
from committed batches only.

Cancellation or an error stops future batches; it does not undo commits already
completed. After an interrupted or limited run, reconcile committed counts and
current state, obtain a fresh preview, and rerun only the still-approved scope.
Do not infer completion from exit status alone if earlier batches committed.
`batch_limit_reached` means the configured maximum was reached; it is not proof
that no candidates remain.

## Stop conditions and reconciliation

Do not execute, or stop before another batch, if the target/scope is uncertain,
delete processing or application quiescence is unverified, a new unknown exclusion
appears, the report does not match the reviewed bounds, or required observation or
recovery evidence is unavailable. Never override an exclusion by changing a
timestamp or weakening the predicate.

No backup/recovery procedure, numerical stop limits, or staffed stage-validation
plan is approved by this document. Obtain those approvals and complete staffed
stage validation before considering production; do not infer them from the sample
defaults or invent them during a run.

The job needs only its normal PostgreSQL connection and standard DML
privileges: it uses CTEs within the existing transaction manager, not temporary
tables, and it creates no indexes or other DDL. No additional index is required
by this job's selectors; any future index change is a separately reviewed
release with its own stage validation and monitoring plan.

After a run, reconcile committed reporter metadata, cascaded reporter history,
common history, distinct roots, removals, surviving shared reporters, exclusions,
and service/database health against the fresh preview. Investigate any difference
before rerunning. This job does not establish stage readiness, production policy,
or a replay/recovery horizon; those require separate validation and approval.

## Local verification

The implementation has passed the repository's `make test` race/short suite and
race-enabled PostgreSQL integration tests for the cleanup job, plus the separate
PostgreSQL identity-artifact contract test. Performance-tagged callers also
pass a compile-only check; no benchmark was executed in that check.

These tests establish local code behavior, including oldest-first bounded
selection with deterministic ties, cancellation, commit-time retry accounting,
shared-root preservation, API revival, and post-cleanup duplicate-ID expiry.
Survivor tests check exact keys and selected state fields, not a full-row hash
of every payload column. They do not establish the operator prerequisites,
stage readiness, or production approval.
