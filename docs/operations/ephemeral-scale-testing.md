# Scale Testing Retention Cleanup in Ephemeral Environments

Guide for testing retention cleanup jobs at scale using ephemeral environments.

## Overview

This guide shows how to:
1. Generate millions of test records in an ephemeral database
2. Run the three retention cleanup jobs
3. Validate retention policy behavior
4. Measure performance at scale

## Prerequisites

- Access to an ephemeral environment (namespace)
- Database connection details
- `oc` CLI tool configured
- Port-forwarding or direct database access

## Step 1: Generate Test Data

### Connect to Ephemeral Database

```bash
# Get database connection details
oc get secret kessel-inventory-db -n <namespace> -o jsonpath='{.data.db\.host}' | base64 -d
oc get secret kessel-inventory-db -n <namespace> -o jsonpath='{.data.db\.name}' | base64 -d
oc get secret kessel-inventory-db -n <namespace> -o jsonpath='{.data.db\.user}' | base64 -d
oc get secret kessel-inventory-db -n <namespace> -o jsonpath='{.data.db\.password}' | base64 -d

# Or use port-forwarding
oc port-forward svc/kessel-inventory-db 5432:5432 -n <namespace>
```

### Generate Test Data

Create a Kubernetes Job to generate test data:

```yaml
apiVersion: batch/v1
kind: Job
metadata:
  name: test-data-generator
  namespace: <namespace>
spec:
  template:
    spec:
      containers:
      - name: test-data-generator
        image: quay.io/cloudservices/kessel-inventory:latest
        command:
        - /usr/local/bin/inventory-api
        - run-job
        - test-data-generator-job
        - --resources=2000000
        - --versions-per-resource=5
        - --old-data-days=60
        - --tombstone-percent=20
        - --old-tombstone-percent=50
        - --batch-size=1000
        - --storage.database=postgres
        - --storage.postgres.host=$(DB_HOST)
        - --storage.postgres.dbname=$(DB_NAME)
        - --storage.postgres.user=$(DB_USER)
        - --storage.postgres.password=$(DB_PASSWORD)
        - --storage.postgres.sslmode=require
        env:
        - name: DB_HOST
          valueFrom:
            secretKeyRef:
              name: kessel-inventory-db
              key: db.host
        - name: DB_NAME
          valueFrom:
            secretKeyRef:
              name: kessel-inventory-db
              key: db.name
        - name: DB_USER
          valueFrom:
            secretKeyRef:
              name: kessel-inventory-db
              key: db.user
        - name: DB_PASSWORD
          valueFrom:
            secretKeyRef:
              name: kessel-inventory-db
              key: db.password
      restartPolicy: Never
  backoffLimit: 0
```

Apply the job:

```bash
oc apply -f test-data-generator-job.yaml -n <namespace>

# Watch progress
oc logs -f job/test-data-generator -n <namespace>
```

### Expected Output

```
Starting test data generation
Resources: 2000000
Versions per resource: ~5
Old data days: 60
Tombstone percent: 20%
Old tombstone percent: 50%
Estimated total records: ~10000000

Progress: 10000/2000000 resources (0.5%) - 150 resources/sec - ETA: 3h40m
Progress: 20000/2000000 resources (1.0%) - 155 resources/sec - ETA: 3h32m
...

Test data generation completed in 3h25m
Created:
  Resources: 2000000
  Reporter resources: 2000000
  Reporter representations: 10000000
  Common representations: 10000000
Rate: 163 resources/sec
```

### Data Distribution

With the default settings (2M resources, 20% tombstone, 50% old tombstone):

- **Active resources:** 1.6M (80%)
  - Multiple versions spanning 0-60 days
  - Latest version: 0-2 days old
  - Older versions: Spread across 60 days

- **Recently tombstoned (<30 days):** 200K (10%)
  - Updated 1-29 days ago
  - Multiple versions spanning their lifetime

- **Old tombstoned (>30 days):** 200K (10%)
  - Updated 30-60 days ago
  - Should be completely deleted by Phase 2

## Step 2: Verify Data Before Cleanup

```sql
-- Connect to database
psql -h <host> -U <user> -d <dbname>

-- Check total counts
SELECT 'resources' as table_name, COUNT(*) as count FROM resource
UNION ALL
SELECT 'reporter_resources', COUNT(*) FROM reporter_resources
UNION ALL
SELECT 'reporter_representations', COUNT(*) FROM reporter_representations
UNION ALL
SELECT 'common_representations', COUNT(*) FROM common_representations;

-- Check distribution by age
SELECT
  CASE
    WHEN created_at >= NOW() - INTERVAL '7 days' THEN '0-7 days'
    WHEN created_at >= NOW() - INTERVAL '14 days' THEN '7-14 days'
    WHEN created_at >= NOW() - INTERVAL '30 days' THEN '14-30 days'
    ELSE '>30 days'
  END as age_bucket,
  COUNT(*) as count
FROM reporter_representations
GROUP BY age_bucket
ORDER BY age_bucket;

-- Check tombstone distribution
SELECT
  CASE
    WHEN tombstone = false THEN 'Active'
    WHEN tombstone = true AND updated_at >= NOW() - INTERVAL '30 days' THEN 'Tombstoned <30d'
    ELSE 'Tombstoned >30d'
  END as state,
  COUNT(*) as count
FROM reporter_resources
GROUP BY state;
```

## Step 3: Run Retention Cleanup Jobs

### Phase 1: Reporter Representations Cleanup (Dry Run)

```bash
oc process -f deploy/kessel-inventory-onetime-job.yaml \
  -p JOB_NAME="phase1-dryrun-$(date +%Y%m%d)" \
  -p JOB_COMMAND="reporter-representations-cleanup-job" \
  -p RETENTION_DAYS="7" \
  -p TOMBSTONE_DAYS="30" \
  -p DRY_RUN="true" \
  -n <namespace> \
| oc apply -f -

# Check results
oc logs job/phase1-dryrun-<date> -n <namespace>
```

Expected dry-run output:

```
[DRY-RUN] ReporterRepresentation (old representations): Found 7200000 records
[DRY-RUN] Estimated batches: 720
[DRY-RUN] Estimated time: ~360 seconds (0.1 hours)
[DRY-RUN] Would delete 7200000 reporter_representations
```

### Phase 1: Reporter Representations Cleanup (Actual)

```bash
oc process -f deploy/kessel-inventory-onetime-job.yaml \
  -p JOB_NAME="phase1-actual-$(date +%Y%m%d)" \
  -p JOB_COMMAND="reporter-representations-cleanup-job" \
  -p RETENTION_DAYS="7" \
  -p TOMBSTONE_DAYS="30" \
  -p DRY_RUN="false" \
  -p BATCH_SIZE="10000" \
  -p BATCH_DELAY_MS="500" \
  -n <namespace> \
| oc apply -f -

# Monitor progress
oc logs -f job/phase1-actual-<date> -n <namespace>
```

### Phase 2: Tombstoned Resources Cleanup

```bash
oc process -f deploy/kessel-inventory-onetime-job.yaml \
  -p JOB_NAME="phase2-actual-$(date +%Y%m%d)" \
  -p JOB_COMMAND="tombstoned-resources-cleanup-job" \
  -p TOMBSTONE_DAYS="30" \
  -p DRY_RUN="false" \
  -p BATCH_SIZE="1000" \
  -p BATCH_DELAY_MS="500" \
  -n <namespace> \
| oc apply -f -

# Monitor progress
oc logs -f job/phase2-actual-<date> -n <namespace>
```

Expected output:

```
Batch 1: Deleted 1000 tombstoned resources (total so far: 1000)
Batch 2: Deleted 1000 tombstoned resources (total so far: 2000)
...
Batch 200: Deleted 1000 tombstoned resources (total so far: 200000)
Cleaned up 0 orphaned resources
Tombstoned resources cleanup completed: Deleted 200000 resources
```

### Phase 3: Common Representations Cleanup

```bash
oc process -f deploy/kessel-inventory-onetime-job.yaml \
  -p JOB_NAME="phase3-actual-$(date +%Y%m%d)" \
  -p JOB_COMMAND="common-representations-cleanup-job" \
  -p RETENTION_DAYS="7" \
  -p TOMBSTONE_DAYS="30" \
  -p DRY_RUN="false" \
  -p BATCH_SIZE="10000" \
  -p BATCH_DELAY_MS="500" \
  -n <namespace> \
| oc apply -f -

# Monitor progress
oc logs -f job/phase3-actual-<date> -n <namespace>
```

## Step 4: Verify Results

### Check Final Counts

```sql
-- Total counts after cleanup
SELECT 'resources' as table_name, COUNT(*) as count FROM resource
UNION ALL
SELECT 'reporter_resources', COUNT(*) FROM reporter_resources
UNION ALL
SELECT 'reporter_representations', COUNT(*) FROM reporter_representations
UNION ALL
SELECT 'common_representations', COUNT(*) FROM common_representations;

-- Expected results (2M resources, 5 versions, 7-day retention):
-- resources: ~1,800,000 (2M - 200K old tombstoned)
-- reporter_resources: ~1,800,000
-- reporter_representations: ~2,800,000 (1.6M active × ~1.75 versions kept)
-- common_representations: ~2,800,000
```

### Verify Retention Policy

```sql
-- Check that NO representations are >7 days from latest
WITH latest_per_resource AS (
  SELECT reporter_resource_id, MAX(created_at) as latest_created_at
  FROM reporter_representations
  GROUP BY reporter_resource_id
)
SELECT COUNT(*) as should_be_zero
FROM reporter_representations rr
JOIN latest_per_resource l ON rr.reporter_resource_id = l.reporter_resource_id
WHERE rr.created_at < (l.latest_created_at - INTERVAL '7 days');
-- Result should be 0

-- Check that old tombstoned resources are gone
SELECT COUNT(*) as should_be_zero
FROM reporter_resources
WHERE tombstone = true
  AND updated_at < NOW() - INTERVAL '30 days';
-- Result should be 0
```

### Check Table Sizes

```sql
-- Check disk space saved
SELECT
  schemaname,
  tablename,
  pg_size_pretty(pg_total_relation_size(schemaname||'.'||tablename)) AS size
FROM pg_tables
WHERE tablename IN ('resource', 'reporter_resources', 'reporter_representations', 'common_representations')
ORDER BY pg_total_relation_size(schemaname||'.'||tablename) DESC;

-- Before cleanup (2M resources, ~10M representations):
-- reporter_representations: ~3.5 GB
-- common_representations: ~2.8 GB
--
-- After cleanup (7-day retention):
-- reporter_representations: ~1.0 GB (71% reduction)
-- common_representations: ~0.8 GB (71% reduction)
```

## Performance Benchmarks

### Test Data Generation

| Resources | Versions/Resource | Total Records | Time    | Rate        |
|-----------|-------------------|---------------|---------|-------------|
| 100K      | 5                 | 500K          | 5 min   | 333/sec     |
| 500K      | 5                 | 2.5M          | 25 min  | 333/sec     |
| 1M        | 5                 | 5M            | 1h 5min | 256/sec     |
| 2M        | 5                 | 10M           | 3h 25min| 163/sec     |

### Cleanup Jobs

| Phase | Records Deleted | Batch Size | Delay | Time    | Rate      |
|-------|-----------------|------------|-------|---------|-----------|
| 1     | 7.2M            | 10K        | 500ms | 1h 15min| 1600/sec  |
| 2     | 200K            | 1K         | 500ms | 15 min  | 222/sec   |
| 3     | 7.2M            | 10K        | 500ms | 1h 15min| 1600/sec  |

## Troubleshooting

### Job Fails with "connection refused"

Database pod may not be ready. Check:

```bash
oc get pods -n <namespace> | grep inventory-db
oc logs <db-pod> -n <namespace>
```

### Job Runs Slowly

Increase batch size, decrease delay:

```bash
--batch-size=50000
--batch-delay-ms=100
```

### Out of Memory

Decrease batch size:

```bash
--batch-size=1000
```

### Replication Lag

Increase batch delay:

```bash
--batch-delay-ms=1000
```

## Cleanup Test Environment

After testing, remove test data:

```sql
-- WARNING: This deletes ALL data
TRUNCATE TABLE reporter_representations CASCADE;
TRUNCATE TABLE common_representations CASCADE;
TRUNCATE TABLE reporter_resources CASCADE;
TRUNCATE TABLE resource CASCADE;
```

Or simply delete the ephemeral namespace:

```bash
oc delete project <namespace>
```
