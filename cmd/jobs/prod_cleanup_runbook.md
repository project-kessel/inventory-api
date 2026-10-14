# Production Cleanup Runbook

Operational guide for running the Kessel Inventory retention cleanup against production.
Everything in this runbook is online and lock-light: no step blocks consumer reads or
writes for more than a moment, and every step is abortable without cleanup work.

Evidence base: isolation test v1.3 (2026-10-13, 2M resources / 10M representations per
namespace) — cleanup + compaction measurably speeds the consumer queries: scenario B
(common-rep retention + compaction) p99 −35%, QPS +13%; scenario A (tombstone + orphan +
compaction of all three tables) p95 −13% with 47% disk reclaimed. The jobs alone do NOT
shrink anything — the physical win comes from the compaction step. Full data:
`retention-cleanup-performance-test-requirements.md` (requirement-docs repo).

## Safety model

| Operation | Lock profile | Can it block prod traffic? |
|---|---|---|
| `CREATE INDEX CONCURRENTLY` | SHARE UPDATE EXCLUSIVE during the build (no DML conflict); brief SHARE locks at start/end can momentarily queue writes | No (if it can't get its locks it waits/fails cleanly) |
| Batched DELETEs (10k + 500ms delay) | Row locks on deleted rows only; MVCC | No — readers never block on row deletes |
| Job temp tables | None (session-local, `pg_temp`, not WAL-logged) | No |
| `pg_repack` | Brief ACCESS EXCLUSIVE at the final swap (typically <1s) | Momentary only; `--wait-timeout` aborts cleanly if it can't swap |
| `ANALYZE` | SHARE UPDATE EXCLUSIVE (blocks DDL/VACUUM, not DML) | No |

Hard rules:

- **Never** `VACUUM FULL` or `TRUNCATE` outside an approved maintenance window — both take
  ACCESS EXCLUSIVE for the whole operation (hours at this scale).
- **Never** plain `CREATE INDEX` (without CONCURRENTLY) on these tables.
- Run at most one cleanup job at a time (`concurrencyPolicy: Forbid` in the CronJob).
- Stopping a job mid-run is always safe: all jobs are batched and idempotent — the next
  run deletes the remainder. `--timeout` makes this a first-class, graceful mechanism.

## Order of operations

```
0. Preflight (read-only)
1. Create indexes (one-time, online)
2. reporter-representations-cleanup-job   (phase 1 — old raw versions)
3. tombstoned-resources-cleanup-job       (phase 2 — deletes old-tombstoned resources;
                                            cascade removes their reporter_representations,
                                            orphans their common_representations)
4. orphaned-common-representations-cleanup-job  (deletes the orphans created by 3)
5. common-representations-cleanup-job     (phase 3 — 7-day retention on the rest)
6. pg_repack the three tables + ANALYZE   (reclaims the disk; THIS is where the
                                            query speedup materializes)
7. Verify
```

Steps 2–5 each need a dry-run first. Steps 2/3/4/5 can be spread across separate
off-peak windows — every job is resumable and order-independent except that 4 must run
after 3 (it cleans up what 3 orphans).

**Experimental alternative (v1.5)**: steps 3+4 can be replaced by a single
`tombstone-cascade-cleanup-job` run, which deletes each old-tombstoned resource's
reporter row and common reps together (batched, with an interleaved orphan pass and a
final drain). It takes `--deletion-order` (`oldest-first`, the default, or
`largest-history-first` — biggest histories die first) plus `--orphan-batch-size`.
It is not yet in the measured evidence base (A/B testing in progress); prefer the
split jobs until it is.

## 0. Preflight (read-only)

```sql
-- Scale of the problem
SELECT relname, n_live_tup, n_dead_tup,
       pg_size_pretty(pg_total_relation_size('public.'||relname)) AS total_size
FROM pg_stat_user_tables
WHERE relname IN ('reporter_resources','reporter_representations','common_representations');

-- Orphaned common reps (the population step 4 will delete)
SELECT COUNT(*) FROM common_representations cr
WHERE NOT EXISTS (SELECT 1 FROM reporter_resources rr WHERE rr.resource_id = cr.resource_id);

-- Old tombstones (the population step 3 will delete)
SELECT COUNT(*) FROM reporter_resources
WHERE tombstone = true AND updated_at < NOW() - INTERVAL '30 days';

-- Replication lag baseline (must be ~0 before starting)
SELECT client_addr, state,
       pg_size_pretty(pg_wal_lsn_diff(pg_current_wal_lsn(), replay_lsn)) AS lag
FROM pg_stat_replication;

-- Scale proxy for the ~2x pg_repack disk requirement: DB size shows the
-- magnitude; actual free headroom is a filesystem/volume check on the DB pod
SELECT pg_size_pretty(pg_database_size(current_database())) AS db_size;
```

Also check the DB pod's ephemeral storage headroom: the jobs' temp-table worklists cost
roughly 24–40 bytes per collected row (~1.5–2 GB at 40M orphans, ~4–6 GB for a 120M-row
retention pass). Temp tables need only the default TEMP privilege (granted to PUBLIC) —
no DBA action is required to use them.

## 1. Indexes (one-time, online)

```sql
-- CRITICAL for tombstoned-resources-cleanup-job: without it every batch seq-scans
-- reporter_resources. Small — covers only tombstoned rows.
CREATE INDEX CONCURRENTLY idx_reporter_resources_tombstoned
  ON reporter_resources (updated_at) WHERE tombstone;

-- Retention-job prerequisites (per retention_cleanup_job.md)
CREATE INDEX CONCURRENTLY idx_common_reps_created_at
  ON common_representations (created_at);
CREATE INDEX CONCURRENTLY idx_reporter_reps_created_at
  ON reporter_representations (created_at);
```

Notes:

- CONCURRENTLY cannot run inside a transaction. If one is interrupted it leaves an
  `INVALID` index — check `pg_index.indisvalid`, `DROP INDEX` and retry.
- It waits for pre-existing long transactions to finish before starting; run when no
  long-running queries are active.
- Already present (no action): `reporter_resource_resource_id_idx`, all three primary
  keys, `reporter_resource_key_idx`, partial `idx_reporter_resources_not_tombstone`.
  The orphan job's anti-join is fully index-backed — it needs no new index.
- Prefer shipping these as schema migrations (the repo's migration framework supports
  CONCURRENTLY for postgres — see `20260326120000_reporter_resources_not_tombstone_idx.go`)
  so they apply with a normal deploy.

## 2–5. Cleanup jobs

Every job: dry-run first, then execute with a graceful `--timeout`.

```bash
# Example: phase 3 (tombstones) — dry-run, then execute with a 24h graceful ceiling
./inventory-api run-job tombstoned-resources-cleanup-job \
  --tombstone-days=30 --dry-run --config .inventory-api.yaml

./inventory-api run-job tombstoned-resources-cleanup-job \
  --tombstone-days=30 \
  --batch-size=1000 --batch-delay-ms=500 \
  --timeout=24h \
  --config .inventory-api.yaml
```

Flags common to all five jobs: `--dry-run`, `--reporter-type`, `--batch-size`,
`--batch-delay-ms`, `--timeout`, `--max-batches` (stop gracefully after N
batches), `--vacuum-every-batches` (online incremental VACUUM, see below).
Phases 2 and 5 additionally take `--retention-days` and `--tombstone-days`;
phase 3 (tombstones) takes only `--tombstone-days`; phase 4 (orphans) takes
neither.

Four of the five jobs delete **oldest data first** (deterministic full-key
ordering); the cascade job's `largest-history-first` variant is the one
exception. Any partial run leaves the newest deletable rows in place (or, for
that variant, the smallest histories) and is resumable.

`--timeout` semantics: the job stops **between batches** once the deadline passes, drops
its temp tables, exits 0, and logs `outcome: partial_timeout` with the deleted-so-far
count. The next run continues from where it stopped. The same graceful exit applies to
`--max-batches` (`partial_max_batches`), a control-table stop (`partial_stopped`), and
SIGTERM/SIGINT (`partial_signal`) — the audit row carries a `stopped_reason` field
naming the trigger. Set the k8s-level `activeDeadlineSeconds` slightly above
`--timeout` as a hard backstop.

### Runtime controls (pause / resume / stop a running job)

Each job checks the `job_runtime_controls` table at every batch boundary. One SQL
statement controls a running job — no pod access or restart needed:

```sql
-- Pause: job parks between batches (worklist preserved), polling every 5s
UPDATE job_runtime_controls SET desired_state = 'paused', updated_by = 'you',
  updated_at = NOW(),
  note = 'investigating consumer p99' WHERE job_name = 'common_representations_cleanup';

-- Resume
UPDATE job_runtime_controls SET desired_state = 'running', updated_by = 'you',
  updated_at = NOW() WHERE job_name = 'common_representations_cleanup';

-- Graceful stop: exits at the next batch boundary (outcome partial_stopped, resumable)
UPDATE job_runtime_controls SET desired_state = 'stop', updated_by = 'you',
  updated_at = NOW() WHERE job_name = 'common_representations_cleanup';
```

Job names: `reporter_representations_cleanup`, `common_representations_cleanup`,
`orphaned_common_representations_cleanup`, `tombstoned_resources_cleanup`,
`tombstone_cascade_cleanup`. The table
ships as migration `20261013120000`; if absent (older schema), jobs warn once and run
with controls disabled — deploy ordering is never a blocker. Any other read error
stops the run gracefully, and a paused job with an unreadable control table stops
after a few failed polls — deletion never resumes on its own (fail-closed: a spurious
stop costs a rerun, an ignored stop costs unwanted deletions). Pre-setting `paused` or
`stop` before a run starts holds/blocks it from the first batch. Note the pause
interval does not count against `--timeout` — keep the k8s
`activeDeadlineSeconds` backstop above your worst-case pause + run time.

Known limitation: no control checks during the one-time collect phase at job start
(read-only, minutes at prod scale). Ensure the k8s termination grace period exceeds
the collect duration, or a pod deletion SIGKILLs mid-collect (harmless — idempotent
rerun recollects).

### Expected durations (extrapolated from v1.3 measurements at 2M scale)

| Job | Measured @ 2M | Extrapolated @ prod scale | Suggested --timeout |
|---|---|---|---|
| tombstoned-resources (850k deleted) | 11m47s | ~10h @ 40M | 24h |
| orphaned-common-reps (4.25M deleted) | 22m17s | ~4h @ 40M orphans | 12h |
| common-reps retention (6M deleted) | 48m13s | ~8–16h @ 60–120M | 24h |
| reporter-reps retention | (untested at scale) | similar to common-reps | 24h |

Deletion rate ≈ 2k rows/s at the default 10k/500ms throttle; the one-time collect scan
at the start adds minutes to tens of minutes at 120M rows (IO-heavy but lock-light).

### Monitoring during execution

```sql
-- Replication lag — the primary throttle indicator
SELECT client_addr, state,
       pg_size_pretty(pg_wal_lsn_diff(pg_current_wal_lsn(), replay_lsn)) AS lag
FROM pg_stat_replication;
```

- **Abort criteria** (stop the job — always safe, always resumable):
  - replication lag sustained above ~10MB and climbing
  - DB CPU saturated / consumer p99 visibly degrading
  - disk filling toward the temp-space requirement
- To slow down or hold the job: prefer the runtime controls above —
  `desired_state = 'paused'` parks it between batches while you investigate, and
  `desired_state = 'stop'` ends the run gracefully. Restarting with
  `--batch-size=5000 --batch-delay-ms=1000` remains the way to change the throttle
  itself.
- Progress is in the job logs (`Batch N: ... total so far: X`); audit log carries
  `action`, `deleted_count`, `outcome` (`success` / `partial_timeout` /
  `partial_stopped` / `partial_signal` / `partial_max_batches` / `failure`) and
  `stopped_reason`.

### Incremental vacuum (optional)

`--vacuum-every-batches N` runs `VACUUM (ANALYZE)` on the job's target tables
every N batches — online (SHARE UPDATE EXCLUSIVE), reclaims reuse-space and refreshes
planner statistics mid-run. It does NOT return disk to the OS; the pg_repack step
below remains required for that. Useful on long runs where dead-tuple accumulation
degrades planner choices; leave off by default.

## 6. Compaction (pg_repack) — where the speedup materializes

Deletes leave dead tuples; autovacuum reclaims reuse-space but the physical table and
index size never shrinks without a rebuild. At v1.3 scale, VACUUM FULL reclaimed 47%
across the three tables; the equivalent pg_repack in prod is what delivers the p99 win.

```sql
-- Availability check first (server-side extension must exist)
SELECT * FROM pg_available_extensions WHERE name = 'pg_repack';
```

```bash
# From a host that can reach the primary, with the pg_repack client CLI installed
# (client and extension versions must match). One table at a time, off-peak.
pg_repack --table reporter_resources        --no-order --wait-timeout 60
pg_repack --table reporter_representations  --no-order --wait-timeout 60
pg_repack --table common_representations    --no-order --wait-timeout 60

# Then refresh planner statistics (online, cheap)
psql -c "ANALYZE reporter_resources; ANALYZE reporter_representations; ANALYZE common_representations;"
```

- **No downtime**: pg_repack rebuilds alongside the live table and swaps at the end.
  The swap holds ACCESS EXCLUSIVE for typically under a second; if it cannot acquire it,
  `--wait-timeout` makes it give up having changed nothing — just re-run later.
- **Disk**: needs ~2× the target table's size free during the rebuild (the new table +
  its indexes). Check before each table.
- Requires a primary key or unique index — all three tables qualify (the two
  representation tables via composite PKs, reporter_resources via its `id` PK
  plus the unique `reporter_resource_key_idx`).
- Heavy sequential IO/CPU during the copy: run off-peak, one table at a time.
- If the prod postgres image does not ship pg_repack: fallback is
  `REINDEX CONCURRENTLY` (fixes index bloat online) plus a maintenance-window
  `VACUUM FULL` for the heap — schedule it explicitly, do not improvise.

## 7. Verify

```sql
-- Orphans gone
SELECT COUNT(*) FROM common_representations cr
WHERE NOT EXISTS (SELECT 1 FROM reporter_resources rr WHERE rr.resource_id = cr.resource_id);

-- Old tombstones gone
SELECT COUNT(*) FROM reporter_resources
WHERE tombstone = true AND updated_at < NOW() - INTERVAL '30 days';

-- Sizes actually shrank (compare to preflight)
SELECT relname, n_live_tup, n_dead_tup,
       pg_size_pretty(pg_total_relation_size('public.'||relname)) AS total_size
FROM pg_stat_user_tables
WHERE relname IN ('reporter_resources','reporter_representations','common_representations');
```

Consumer query impact: watch the existing service dashboards (Q1 = FindLatestRepresentations,
Q2 = FindCurrentAndPreviousVersionedRepresentations p99) — the v1.3 isolation runs are the
expected shape: small transient noise during deletion, durable improvement after compaction.

## app-interface CronJob configuration

The timeout value lives in the container args, so app-interface is the configuration
point. Suggested per-job values (tune after the first measured prod run):

```yaml
apiVersion: batch/v1
kind: CronJob
metadata:
  name: inventory-tombstoned-resources-cleanup
spec:
  schedule: "0 2 * * *"            # off-peak; jobs are resumable so a missed window is safe
  concurrencyPolicy: Forbid        # never two instances of the same job
  successfulJobsHistoryLimit: 3
  failedJobsHistoryLimit: 3
  jobTemplate:
    spec:
      backoffLimit: 0              # a failed run is investigated, not blind-retried
      activeDeadlineSeconds: 90000 # ~25h — hard backstop, slightly above --timeout=24h
      template:
        spec:
          restartPolicy: Never
          containers:
          - name: cleanup
            image: quay.io/project-kessel/inventory-api:latest
            args:
            - run-job
            - tombstoned-resources-cleanup-job
            - --tombstone-days=30
            - --batch-size=1000
            - --batch-delay-ms=500
            - --timeout=24h
```

Repeat per job with its own schedule window and timeout (orphan: `--timeout=12h`,
`activeDeadlineSeconds: 46800`; common-reps: `--timeout=24h`). Stagger the schedules so
the jobs never overlap.

## Known limitations / follow-ups

- The orphan job deletes representations but leaves parent `resource` rows with no
  reporter_resources reference (acknowledged TODO in `retention_helpers.go`).
- `--tombstone-days` on the common-representations job is accepted for interface
  symmetry but NOT applied by its SQL (now documented in the job's `--help` and
  pinned by a test); old-tombstone common reps are instead handled by jobs 3+4.
- Single-iteration perf numbers: repeat runs before quoting exact percentages in reviews
  (v1.3 caveats section).
