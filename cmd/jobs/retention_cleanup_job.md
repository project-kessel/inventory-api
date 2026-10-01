# Retention Cleanup Job

Batch deletion job for removing old representation data based on retention policy. Deletes historical version data from `reporter_representations` and `common_representations` tables while preserving current resource and reporter_resource records.

## Purpose

This job implements a time-based retention policy for historical representation data. It removes old versions while keeping:
- ✅ All `resource` records (the resources themselves)
- ✅ All `reporter_resources` records (current resource metadata)
- ❌ Old `reporter_representations` records (historical raw data versions)
- ❌ Old `common_representations` records (historical normalized data versions)

## Prerequisites

### 1. Create Database Indexes (CRITICAL)

**Before running this job in production**, create indexes on `created_at` columns to ensure reasonable performance:

```sql
-- These indexes are REQUIRED for acceptable performance
-- Run with CONCURRENTLY to avoid blocking production traffic
CREATE INDEX CONCURRENTLY idx_reporter_reps_created_at 
  ON reporter_representations(created_at);

CREATE INDEX CONCURRENTLY idx_common_reps_created_at 
  ON common_representations(created_at);
```

**Without these indexes**: Deletion will take days/weeks instead of hours.
**With indexes**: Deletion takes 20-40 hours for 100M+ rows.

Index creation time: ~30-90 minutes per index (non-blocking).

### 2. Verify Data Distribution

Check how much data will be deleted:

```sql
-- Check reporter_representations
SELECT 
  COUNT(*) as total_rows,
  COUNT(*) FILTER (WHERE created_at < NOW() - INTERVAL '7 days') as to_delete,
  MIN(created_at) as oldest,
  MAX(created_at) as newest
FROM reporter_representations;

-- Check common_representations
SELECT 
  COUNT(*) as total_rows,
  COUNT(*) FILTER (WHERE created_at < NOW() - INTERVAL '7 days') as to_delete,
  MIN(created_at) as oldest,
  MAX(created_at) as newest
FROM common_representations;
```

## Usage

### Step 1: Dry-Run (ALWAYS run this first)

```bash
./inventory-api run-job retention-cleanup-job \
  --retention-days=7 \
  --dry-run \
  --config .inventory-api.yaml
```

This will show:
- Number of records that would be deleted
- Estimated number of batches
- Estimated time based on batch delays

### Step 2: Execute Deletion

**For all reporters:**

```bash
./inventory-api run-job retention-cleanup-job \
  --retention-days=7 \
  --batch-size=10000 \
  --batch-delay-ms=500 \
  --config .inventory-api.yaml
```

**For specific reporter only:**

```bash
./inventory-api run-job retention-cleanup-job \
  --retention-days=7 \
  --reporter-type=hbi \
  --batch-size=10000 \
  --batch-delay-ms=500 \
  --config .inventory-api.yaml
```

### Step 3: Monitor Progress

Watch the logs for progress updates:

```bash
# Logs show progress every batch
Batch 1234: Deleted 10000 ReporterRepresentation records (total so far: 12340000)
```

Query the database to check remaining rows:

```sql
-- Check progress
SELECT COUNT(*) as remaining
FROM reporter_representations
WHERE created_at < NOW() - INTERVAL '7 days';

SELECT COUNT(*) as remaining
FROM common_representations
WHERE created_at < NOW() - INTERVAL '7 days';
```

### Step 4: Reclaim Disk Space (After Deletion)

After the job completes, run VACUUM to reclaim disk space:

```sql
-- Run during off-peak hours (can take several hours)
VACUUM ANALYZE reporter_representations;
VACUUM ANALYZE common_representations;
```

## Configuration Options

| Flag | Default | Description |
|------|---------|-------------|
| `--retention-days` | 7 | Delete records older than this many days |
| `--reporter-type` | (empty) | Only delete for specific reporter. If empty, deletes for all reporters |
| `--batch-size` | 10000 | Number of records to delete per batch |
| `--batch-delay-ms` | 500 | Milliseconds to wait between batches |
| `--dry-run` | false | Preview counts without deleting |

### Tuning Parameters

**For faster deletion** (higher database load):
```bash
--batch-size=20000 \
--batch-delay-ms=100
```

**For lower database impact** (slower deletion):
```bash
--batch-size=5000 \
--batch-delay-ms=1000
```

## Safety Features

✅ **Batched deletion**: Deletes in small chunks to avoid long locks
✅ **Throttled**: Configurable delay between batches prevents DB overload  
✅ **Resumable**: Can stop and restart safely
✅ **Dry-run mode**: Preview before executing
✅ **Progress logging**: Reports progress every batch
✅ **Idempotent**: Safe to run multiple times
✅ **Non-destructive to core data**: Only deletes historical versions, not resources

## Delete Phases

The job runs in two phases:

1. **Phase 1: ReporterRepresentations**
   - Deletes old `reporter_representations` records based on `created_at < cutoff_date`
   - Uses composite primary key `(reporter_resource_id, version, generation)` for batch deletion
   - Optionally filters by `reporter_type` if specified

2. **Phase 2: CommonRepresentations**
   - Deletes old `common_representations` records based on `created_at < cutoff_date`
   - Uses composite primary key `(resource_id, version)` for batch deletion
   - Optionally filters by `reported_by_reporter_type` if specified

## If the Job Fails

**Safe to re-run**: The job is idempotent. If it fails midway:

1. Check logs to identify which phase failed
2. Fix the underlying issue (connection, disk space, replication lag, etc.)
3. Re-run the exact same command
4. Job will pick up where it left off (only deletes remaining records)

**Common failure scenarios:**

- **Replication lag**: Reduce batch size or increase delay
- **Disk space**: Ensure enough space for WAL logs
- **Connection timeout**: Job can be restarted; batches are atomic

## Performance Estimates

Based on production data with ~120M reporter_representations and ~90M common_representations:

**With indexes:**
- Batch size: 10,000 rows
- Batch delay: 500ms
- Expected batches: ~21,000 total (120M + 90M) / 10k
- Estimated time: **25-40 hours**

**Without indexes:**
- Not recommended (would take weeks)

## Production Deployment

### As a Kubernetes CronJob

```yaml
apiVersion: batch/v1
kind: CronJob
metadata:
  name: inventory-retention-cleanup
  namespace: kessel-prod
spec:
  schedule: "0 2 * * *"  # Daily at 2 AM
  concurrencyPolicy: Forbid  # Don't run multiple instances
  successfulJobsHistoryLimit: 3
  failedJobsHistoryLimit: 3
  jobTemplate:
    spec:
      backoffLimit: 2  # Retry twice on failure
      activeDeadlineSeconds: 172800  # 48 hour timeout
      template:
        metadata:
          labels:
            app: inventory-retention-cleanup
        spec:
          restartPolicy: OnFailure
          containers:
          - name: retention-cleanup
            image: quay.io/project-kessel/inventory-api:latest
            command:
            - /inventory-api
            - run-job
            - retention-cleanup-job
            - --retention-days=7
            - --batch-size=10000
            - --batch-delay-ms=500
            - --config=/etc/inventory-api/config.yaml
            env:
            - name: DB_HOST
              valueFrom:
                secretKeyRef:
                  name: inventory-db
                  key: host
            - name: DB_PASSWORD
              valueFrom:
                secretKeyRef:
                  name: inventory-db
                  key: password
            resources:
              requests:
                memory: "256Mi"
                cpu: "200m"
              limits:
                memory: "512Mi"
                cpu: "500m"
            volumeMounts:
            - name: config
              mountPath: /etc/inventory-api
          volumes:
          - name: config
            configMap:
              name: inventory-api-config
```

### Monitoring

Monitor these metrics:

- **Replication lag**: Should stay under 10MB during deletion
- **Disk space**: Ensure adequate space for WAL logs
- **Job duration**: Should complete within expected time window
- **Deleted row counts**: Should match dry-run estimates

```sql
-- Check replication lag (on primary)
SELECT 
  client_addr,
  state,
  pg_wal_lsn_diff(pg_current_wal_lsn(), replay_lsn) as lag_bytes,
  pg_size_pretty(pg_wal_lsn_diff(pg_current_wal_lsn(), replay_lsn)) as lag
FROM pg_stat_replication;

-- Check table bloat (after deletion, before VACUUM)
SELECT 
  schemaname,
  tablename,
  pg_size_pretty(pg_total_relation_size(schemaname||'.'||tablename)) as total_size,
  n_dead_tup as dead_rows
FROM pg_stat_user_tables
WHERE tablename IN ('reporter_representations', 'common_representations');
```

## Example Output

### Dry-Run

```
INFO Starting retention cleanup job for ALL reporters, retention=7 days
INFO Cutoff date: 2026-09-24T00:00:00Z (will delete records created before this date)
INFO [DRY-RUN] ReporterRepresentation: Found 118456234 records
INFO [DRY-RUN] Estimated batches: 11846
INFO [DRY-RUN] Estimated time: ~5923 seconds (1.6 hours). This is based on batch delays only and does not account for actual deletion time.
INFO [DRY-RUN] CommonRepresentation: Found 89234567 records  
INFO [DRY-RUN] Estimated batches: 8924
INFO [DRY-RUN] Estimated time: ~4462 seconds (1.2 hours). This is based on batch delays only and does not account for actual deletion time.
INFO [DRY-RUN] Summary: Would delete ReporterRepresentation=118456234, CommonRepresentation=89234567
INFO [DRY-RUN] No data was modified
```

### Actual Execution

```
INFO Starting retention cleanup job for ALL reporters, retention=7 days
INFO Cutoff date: 2026-09-24T00:00:00Z (will delete records created before this date)
INFO Using batch size: 10000 rows, delay between batches: 500ms
INFO Starting batched deletion of ReporterRepresentation records...
INFO Batch 1: Deleted 10000 ReporterRepresentation records (total so far: 10000)
INFO Batch 2: Deleted 10000 ReporterRepresentation records (total so far: 20000)
...
INFO Batch 11846: Deleted 6234 ReporterRepresentation records (total so far: 118456234)
INFO Completed: Deleted 118456234 total ReporterRepresentation records
INFO Starting batched deletion of CommonRepresentation records...
INFO Batch 1: Deleted 10000 CommonRepresentation records (total so far: 10000)
...
INFO Batch 8924: Deleted 4567 CommonRepresentation records (total so far: 89234567)
INFO Completed: Deleted 89234567 total CommonRepresentation records
INFO Retention cleanup job completed successfully. Total records deleted: ReporterRepresentation=118456234, CommonRepresentation=89234567
```

## Troubleshooting

### Deletion is very slow

**Cause**: Missing indexes on `created_at` columns
**Solution**: Create indexes (see Prerequisites section)

### Replication lag building up

**Cause**: Batch size too large or delay too short
**Solution**: Reduce batch size to 5000 and increase delay to 1000ms

### Job times out

**Cause**: Very large dataset and strict timeout
**Solution**: Increase `activeDeadlineSeconds` in CronJob or run manually in batches

### Out of disk space

**Cause**: WAL logs accumulating faster than they can be archived
**Solution**: 
1. Ensure adequate disk space (50GB+ free recommended)
2. Reduce batch size to slow down WAL generation
3. Check WAL archiving is working properly

## FAQ

**Q: Does this delete resources?**
A: No, only old representation data (version history).

**Q: Can I run this during business hours?**
A: Yes, with appropriate batch size and delay. Monitor replication lag.

**Q: How often should this run?**
A: Daily is recommended to prevent accumulation.

**Q: Can I have different retention for different reporters?**
A: Not yet, but this can be added in the future. Currently, use `--reporter-type` flag and run separate jobs.

**Q: What happens if I need to restore deleted data?**
A: Deleted data cannot be restored. Ensure dry-run results are acceptable before executing.

**Q: Can this job run concurrently?**
A: No, use `concurrencyPolicy: Forbid` in CronJob to prevent overlapping executions.
