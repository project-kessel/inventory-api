# Retention Cleanup Job - Production Deployment Guide

This guide covers deploying and running the retention cleanup job in production environments.

## Overview

The retention cleanup job can run in two modes:
1. **Manual/One-time**: For initial cleanup or testing
2. **Scheduled (CronJob)**: For ongoing daily maintenance

## Prerequisites

- ✅ Database indexes created (see PR #1509)
- ✅ Retention cleanup job deployed (see PR #1508)
- ✅ Access to production OpenShift cluster
- ✅ Sufficient database disk space (50GB+ free recommended)

---

## Phase 1: Initial Cleanup (Manual)

The initial cleanup removes ~8 months of historical data (~204M rows, ~65GB).

### Step 1: Dry-Run First (ALWAYS)

```bash
# Process template and create dry-run job
oc process -f deploy/kessel-inventory-onetime-job.yaml \
  -p JOB_NAME="retention-cleanup-initial-dryrun-$(date +%Y%m%d)" \
  -p JOB_COMMAND="retention-cleanup-job" \
  -p RESOURCE_TYPE="" \
  -p REPORTER_TYPE="" \
  -p DRY_RUN="true" \
  -p BATCH_SIZE="10000" \
  -p BATCH_DELAY_MS="500" \
  -p IMAGE_TAG="v1.2.3" \
  -p REQUESTS_MEMORY="512Mi" \
  -p LIMITS_MEMORY="1Gi" \
| oc apply -f -

# Watch the dry-run output
oc logs -f job/retention-cleanup-initial-dryrun-$(date +%Y%m%d)
```

**Expected output:**
```
INFO Starting retention cleanup job for ALL reporters, retention=7 days
INFO Cutoff date: 2026-09-24T00:00:00Z (will delete records created before this date)
[DRY-RUN] ReporterRepresentation (older than cutoff): Found 118456234 records
[DRY-RUN] Estimated batches: 11846
[DRY-RUN] Estimated time: ~5923 seconds (1.6 hours). This is based on batch delays only...
[DRY-RUN] CommonRepresentation (older than cutoff): Found 89234567 records
[DRY-RUN] Estimated batches: 8924
[DRY-RUN] Estimated time: ~4462 seconds (1.2 hours)...
[DRY-RUN] Summary: Would delete ReporterRepresentation=118456234, CommonRepresentation=89234567
[DRY-RUN] No data was modified
```

**Verify:**
- Row counts match expectations (~118M + ~89M = ~207M)
- Estimated time seems reasonable (10-20 hours with indexes)
- No errors in output

### Step 2: Execute Actual Cleanup

```bash
# Create the actual cleanup job
oc process -f deploy/kessel-inventory-onetime-job.yaml \
  -p JOB_NAME="retention-cleanup-initial-$(date +%Y%m%d)" \
  -p JOB_COMMAND="retention-cleanup-job" \
  -p RESOURCE_TYPE="" \
  -p REPORTER_TYPE="" \
  -p DRY_RUN="false" \
  -p BATCH_SIZE="10000" \
  -p BATCH_DELAY_MS="500" \
  -p IMAGE_TAG="v1.2.3" \
  -p REQUESTS_MEMORY="512Mi" \
  -p REQUESTS_CPU="200m" \
  -p LIMITS_MEMORY="1Gi" \
  -p LIMITS_CPU="500m" \
| oc apply -f -

# Monitor progress (in separate terminal)
oc logs -f job/retention-cleanup-initial-$(date +%Y%m%d)
```

**Expected duration:** 40-72 hours (recommend starting Friday evening for weekend run)

### Step 3: Monitor During Execution

**Check job status:**
```bash
# Job status
oc get job retention-cleanup-initial-$(date +%Y%m%d)

# Pod status
oc get pods -l job-type=retention-cleanup-initial
```

**Monitor progress in logs:**
```bash
# Watch for progress updates (every 10 batches)
oc logs -f job/retention-cleanup-initial-$(date +%Y%m%d) | grep "Batch"

# Example output:
# Batch 100: Deleted 10000 ReporterRepresentation records (total so far: 1000000)
# Batch 200: Deleted 10000 ReporterRepresentation records (total so far: 2000000)
```

**Check database metrics:**
```sql
-- In psql session on database
-- Check remaining rows to delete
SELECT COUNT(*) as remaining
FROM reporter_representations
WHERE created_at < NOW() - INTERVAL '7 days';

-- Monitor replication lag (should stay < 10MB)
SELECT 
  client_addr,
  pg_wal_lsn_diff(pg_current_wal_lsn(), replay_lsn) as lag_bytes,
  pg_size_pretty(pg_wal_lsn_diff(pg_current_wal_lsn(), replay_lsn)) as lag
FROM pg_stat_replication;
```

### Step 4: Verify Completion

```bash
# Check if job completed successfully
oc get job retention-cleanup-initial-$(date +%Y%m%d)
# Should show: COMPLETIONS: 1/1

# Check final logs
oc logs job/retention-cleanup-initial-$(date +%Y%m%d) | tail -20
```

**Expected final output:**
```
INFO Completed: Deleted 118456234 total ReporterRepresentation records
INFO Completed: Deleted 89234567 total CommonRepresentation records
INFO Retention cleanup job completed successfully. Total records deleted: ReporterRepresentation=118456234, CommonRepresentation=89234567
```

### Step 5: Reclaim Disk Space (VACUUM)

After deletion, run VACUUM to reclaim disk space:

```sql
-- Run during off-peak hours (can take 4-8 hours)
VACUUM ANALYZE reporter_representations;
VACUUM ANALYZE common_representations;

-- Check space reclaimed
SELECT 
  schemaname,
  tablename,
  pg_size_pretty(pg_total_relation_size(schemaname||'.'||tablename)) as total_size,
  n_dead_tup as dead_rows
FROM pg_stat_user_tables
WHERE tablename IN ('reporter_representations', 'common_representations');
```

---

## Phase 2: Scheduled Daily Cleanup (CronJob)

After initial cleanup, deploy a CronJob for daily maintenance.

### Step 1: Deploy CronJob

```bash
# Deploy the CronJob (starts suspended for verification)
oc process -f deploy/kessel-inventory-retention-cleanup-cronjob.yaml \
  -p IMAGE_TAG="v1.2.3" \
  -p RETENTION_SCHEDULE="0 2 * * *" \
  -p RETENTION_DAYS="7" \
  -p BATCH_SIZE="10000" \
  -p BATCH_DELAY_MS="500" \
  -p RETENTION_CRONJOB_SUSPEND="true" \
  -p RETENTION_JOB_TIMEOUT="172800" \
| oc apply -f -

# Verify CronJob created
oc get cronjob retention-cleanup
```

### Step 2: Test Manual Trigger

Before enabling automatic schedule, test manually:

```bash
# Manually trigger the CronJob once
oc create job --from=cronjob/retention-cleanup retention-cleanup-test-$(date +%Y%m%d)

# Watch the test run
oc logs -f job/retention-cleanup-test-$(date +%Y%m%d)
```

**Expected daily cleanup (after initial cleanup):**
- Rows to delete: ~0.9-1.5M per day (just yesterday's data that's now >7 days old)
- Time: 15-30 minutes
- Should complete quickly

### Step 3: Enable Scheduled Execution

Once manual test succeeds:

```bash
# Enable the CronJob
oc patch cronjob retention-cleanup -p '{"spec":{"suspend":false}}'

# Verify it's active
oc get cronjob retention-cleanup
# SUSPEND should show "False"
```

### Step 4: Monitor Scheduled Runs

**Check CronJob schedule:**
```bash
# View upcoming schedule
oc get cronjob retention-cleanup
# Shows LAST SCHEDULE and next run time

# View job history
oc get jobs -l job-type=retention-cleanup --sort-by=.metadata.creationTimestamp
```

**Check recent execution:**
```bash
# Get most recent job
LATEST_JOB=$(oc get jobs -l job-type=retention-cleanup --sort-by=.metadata.creationTimestamp -o name | tail -1)

# Check status
oc get $LATEST_JOB

# View logs
oc logs $LATEST_JOB
```

---

## Configuration Options

### CronJob Schedule Patterns

```yaml
# Every day at 2 AM UTC
schedule: "0 2 * * *"

# Every day at 3:30 AM UTC
schedule: "30 3 * * *"

# Every Sunday at 1 AM UTC (weekly)
schedule: "0 1 * * 0"

# Shorthand for common schedules
schedule: "@daily"    # Midnight UTC
schedule: "@weekly"   # Sunday midnight UTC
schedule: "@monthly"  # 1st of month midnight UTC
```

### Tuning Parameters

**Conservative (lower DB impact):**
```bash
-p BATCH_SIZE="5000"
-p BATCH_DELAY_MS="1000"
```

**Balanced (default):**
```bash
-p BATCH_SIZE="10000"
-p BATCH_DELAY_MS="500"
```

**Aggressive (faster, higher DB impact):**
```bash
-p BATCH_SIZE="20000"
-p BATCH_DELAY_MS="100"
```

**Different retention periods:**
```bash
# 7 days (default)
-p RETENTION_DAYS="7"

# 14 days
-p RETENTION_DAYS="14"

# 30 days
-p RETENTION_DAYS="30"
```

**Filter by reporter:**
```bash
# All reporters (default)
-p REPORTER_TYPE=""

# Only HBI
-p REPORTER_TYPE="hbi"

# Only OCM
-p REPORTER_TYPE="ocm"
```

---

## Troubleshooting

### Job Fails or Times Out

**Check logs:**
```bash
oc logs job/<job-name> | grep -i error
```

**Common issues:**
- Replication lag too high → Increase batch delay
- Database connection timeout → Job will retry
- Out of memory → Increase memory limits

**Increase timeout:**
```bash
# Increase to 72 hours (259200 seconds)
oc patch cronjob retention-cleanup -p '{"spec":{"jobTemplate":{"spec":{"activeDeadlineSeconds":259200}}}}'
```

### Job Runs Too Slowly

**Speed up:**
```bash
# Increase batch size, decrease delay
oc patch cronjob retention-cleanup -p '{
  "spec":{
    "jobTemplate":{
      "spec":{
        "template":{
          "spec":{
            "containers":[{
              "name":"retention-cleanup",
              "command":[
                "inventory-api",
                "run-job",
                "retention-cleanup-job",
                "--retention-days=7",
                "--batch-size=20000",
                "--batch-delay-ms=100"
              ]
            }]
          }
        }
      }
    }
  }
}'
```

### Disk Space Issues

**Check free space before starting:**
```sql
SELECT pg_size_pretty(pg_database_size('inventory')) as db_size;
```

Ensure at least 50GB free for WAL logs during deletion.

### Replication Lag Building Up

**Check lag:**
```sql
SELECT pg_size_pretty(pg_wal_lsn_diff(pg_current_wal_lsn(), replay_lsn))
FROM pg_stat_replication;
```

**If lag > 100MB:**
- Stop the job
- Let replication catch up
- Restart with lower batch size or higher delay

---

## Monitoring and Alerts

### Metrics to Track

1. **Job completion status**
   - Success/failure rate
   - Duration per run
   - Rows deleted per run

2. **Database metrics**
   - Table sizes (should stay ~1-2GB after initial cleanup)
   - Replication lag during execution
   - Dead tuple count

3. **Performance metrics**
   - Consumer query latency (should improve after cleanup)
   - Buffer cache hit rate (should improve)

### Alerting Rules

**Recommended alerts:**
- Job fails 2 times in a row → Page on-call
- Job duration > 2 hours (for daily runs) → Investigate
- Replication lag > 100MB during execution → Warning
- Table size growth > 5GB → Review retention policy

---

## Rollback / Emergency Procedures

### Suspend CronJob Immediately

```bash
# Stop all future runs
oc patch cronjob retention-cleanup -p '{"spec":{"suspend":true}}'
```

### Stop Running Job

```bash
# Delete the active job (will stop gracefully)
oc delete job <job-name>
```

### Restore from Backup

If data was accidentally deleted:
```bash
# Database backups should be handled by existing backup procedures
# Contact DBA team for point-in-time recovery
```

---

## Deployment Checklist

**Initial cleanup:**
- [ ] Indexes created on created_at columns
- [ ] Dry-run executed and verified
- [ ] Disk space checked (50GB+ free)
- [ ] Start time scheduled (off-peak, weekend)
- [ ] Monitoring in place
- [ ] Actual cleanup job started
- [ ] Completion verified
- [ ] VACUUM executed
- [ ] Space reclaimed verified

**Scheduled cleanup:**
- [ ] CronJob template deployed (suspended)
- [ ] Manual test job executed successfully
- [ ] CronJob enabled (suspend=false)
- [ ] First scheduled run verified
- [ ] Alerts configured
- [ ] Runbook documented

---

## Reference

- **Retention Cleanup Job Documentation**: `cmd/jobs/retention_cleanup_job.md`
- **Design Decision Record**: `docs/design/retention-cleanup-indexes-ddr.md`
- **PR #1508**: Retention cleanup job
- **PR #1509**: Database indexes for retention cleanup
