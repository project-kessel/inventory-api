# Retention Cleanup Jobs

Three independent jobs for cleaning up historical representation data, ordered by production impact.

## Overview

The retention cleanup process is split into three phases that can be run independently:

1. **reporter-representations-cleanup-job** (Phase 1 - Lowest Impact)
2. **tombstoned-resources-cleanup-job** (Phase 2 - Medium Impact)  
3. **common-representations-cleanup-job** (Phase 3 - Highest Impact)

## Phase 1: Reporter Representations Cleanup

**Impact:** Lowest - Reporter-specific versioned data, not queried by consumers

### Usage

```bash
# Dry-run to preview deletion
./bin/inventory-api run-job reporter-representations-cleanup-job \
  --dry-run \
  --retention-days 7 \
  --tombstone-days 30 \
  --storage.database postgres \
  --storage.postgres.host localhost

# Actual deletion
./bin/inventory-api run-job reporter-representations-cleanup-job \
  --retention-days 7 \
  --tombstone-days 30 \
  --batch-size 10000 \
  --batch-delay-ms 500 \
  --storage.database postgres \
  --storage.postgres.host localhost
```

### What it deletes

- Reporter representations older than N days from latest per reporter_resource
- Excludes tombstoned resources >M days (handled by Phase 2)
- Active + recently tombstoned resources only

### Kubernetes Job

```bash
oc process -f deploy/kessel-inventory-onetime-job.yaml \
  -p JOB_NAME="reporter-reps-cleanup-$(date +%Y%m%d)" \
  -p JOB_COMMAND="reporter-representations-cleanup-job" \
  -p RETENTION_DAYS="7" \
  -p TOMBSTONE_DAYS="30" \
  -p DRY_RUN="false" \
  -n kessel-prod \
| oc apply -f -
```

## Phase 2: Tombstoned Resources Cleanup

**Impact:** Medium - Deletes resources already marked as deleted, cascades to representations

### Usage

```bash
# Dry-run to preview deletion
./bin/inventory-api run-job tombstoned-resources-cleanup-job \
  --dry-run \
  --tombstone-days 30 \
  --storage.database postgres \
  --storage.postgres.host localhost

# Actual deletion
./bin/inventory-api run-job tombstoned-resources-cleanup-job \
  --tombstone-days 30 \
  --batch-size 1000 \
  --batch-delay-ms 500 \
  --storage.database postgres \
  --storage.postgres.host localhost
```

### What it deletes

- `reporter_resources` records where `tombstone=true AND updated_at < NOW() - M days`
- All associated `reporter_representations` via cascade delete
- Orphaned `resource` records (no reporter_resources reference them)

### Kubernetes Job

```bash
oc process -f deploy/kessel-inventory-onetime-job.yaml \
  -p JOB_NAME="tombstoned-cleanup-$(date +%Y%m%d)" \
  -p JOB_COMMAND="tombstoned-resources-cleanup-job" \
  -p TOMBSTONE_DAYS="30" \
  -p DRY_RUN="false" \
  -n kessel-prod \
| oc apply -f -
```

## Phase 3: Common Representations Cleanup

**Impact:** Highest - Consumer queries read from this table

### Usage

```bash
# Dry-run to preview deletion
./bin/inventory-api run-job common-representations-cleanup-job \
  --dry-run \
  --retention-days 7 \
  --tombstone-days 30 \
  --storage.database postgres \
  --storage.postgres.host localhost

# Actual deletion
./bin/inventory-api run-job common-representations-cleanup-job \
  --retention-days 7 \
  --tombstone-days 30 \
  --batch-size 10000 \
  --batch-delay-ms 500 \
  --storage.database postgres \
  --storage.postgres.host localhost
```

### What it deletes

- Common representations older than N days from latest per resource
- Excludes tombstoned resources >M days (already deleted by Phase 2)
- Active + recently tombstoned resources only

### Kubernetes Job

```bash
oc process -f deploy/kessel-inventory-onetime-job.yaml \
  -p JOB_NAME="common-reps-cleanup-$(date +%Y%m%d)" \
  -p JOB_COMMAND="common-representations-cleanup-job" \
  -p RETENTION_DAYS="7" \
  -p TOMBSTONE_DAYS="30" \
  -p DRY_RUN="false" \
  -n kessel-prod \
| oc apply -f -
```

## Recommended Execution Order

### One-Time Cleanup (Production)

Run phases sequentially with monitoring between each:

```bash
# 1. Phase 1: Reporter representations (safest)
oc apply -f phase1-job.yaml
# Monitor for 1-2 hours, check database metrics, consumer latency

# 2. Phase 2: Tombstoned resources (medium risk)
oc apply -f phase2-job.yaml
# Monitor for 1-2 hours, check cascade deletes completed

# 3. Phase 3: Common representations (highest risk)
oc apply -f phase3-job.yaml
# Monitor closely, check consumer query performance
```

### Scheduled Maintenance (Future)

After successful one-time cleanup, can schedule as CronJobs:

```yaml
# Run all three sequentially once per week
# Phase 1: Sunday 2 AM
# Phase 2: Sunday 4 AM (after Phase 1 completes)
# Phase 3: Sunday 6 AM (after Phase 2 completes)
```

## Common Flags

All jobs support:

- `--dry-run` - Preview deletion counts without modifying data
- `--retention-days N` - Delete representations >N days from latest (default: 7)
- `--tombstone-days M` - Tombstone threshold in days (default: 30)
- `--reporter-type TYPE` - Filter by reporter type (e.g., "hbi")
- `--batch-size N` - Records per batch (default: 10000 for reps, 1000 for resources)
- `--batch-delay-ms N` - Delay between batches in milliseconds (default: 500)

## Monitoring

### During Execution

```bash
# Watch job logs
oc logs -f job/reporter-reps-cleanup-20261002

# Monitor database load
# Check pg_stat_activity, replication lag, consumer query latency

# Check deletion progress
# Job logs show batch counts and total deleted
```

### Database Queries

```sql
-- Check remaining old data
SELECT COUNT(*) FROM reporter_representations 
WHERE created_at < NOW() - INTERVAL '7 days';

-- Check tombstoned resources
SELECT COUNT(*) FROM reporter_resources 
WHERE tombstone = true AND updated_at < NOW() - INTERVAL '30 days';

-- Check table sizes
SELECT pg_size_pretty(pg_total_relation_size('reporter_representations'));
SELECT pg_size_pretty(pg_total_relation_size('common_representations'));
```

## Stopping/Resuming

Jobs can be stopped safely and will resume from where they left off:

```bash
# Stop job
oc delete job reporter-reps-cleanup-20261002

# Resume with same parameters
oc apply -f same-job.yaml
```

Deletion is batched and atomic per batch, so partial completion is safe.

## Safety Features

- **Relative retention:** Deletes N days from latest per resource, not absolute date
- **Current version protection:** Never deletes current or current-1 versions
- **Tombstone exclusion:** Old tombstoned resources handled separately
- **Batched deletion:** Prevents long-running transactions
- **Configurable delays:** Reduces database load between batches
- **Dry-run mode:** Preview impact before execution

## Troubleshooting

### Job times out

Increase `--batch-delay-ms` to reduce database load, or decrease `--batch-size` for smaller batches.

### Replication lag increases

Pause job, wait for lag to catch up, resume with higher `--batch-delay-ms`.

### Consumer queries slow during Phase 3

Expected due to table churn. Monitor and stop if latency exceeds SLO. Resume during low-traffic window.

### "No records deleted" but dry-run shows data

Check tombstone exclusion logic. Old tombstoned resources require Phase 2 first.
