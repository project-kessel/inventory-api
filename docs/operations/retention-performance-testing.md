# Retention Cleanup Performance Testing Guide

Comprehensive guide for testing the impact of retention cleanup on query performance at scale.

## Overview

This test validates that:
1. **Large database size degrades query performance** - Establishes baseline with millions of records
2. **Retention cleanup improves query performance** - Measures improvement after each cleanup phase
3. **Cleanup is safe under load** - Queries continue working during and after cleanup

## Test Flow

```
1. Generate Test Data (2M resources, ~10M representations)
   ↓
2. Baseline Performance Test (200K queries, measure p99 latency)
   ↓
3. Phase 1: Cleanup reporter_representations
   ↓
4. Performance Test (measure improvement)
   ↓
5. Phase 2: Cleanup tombstoned resources
   ↓
6. Performance Test (measure improvement)
   ↓
7. Phase 3: Cleanup common_representations
   ↓
8. Final Performance Test (measure total improvement)
   ↓
9. Generate Detailed Report
```

## Queries Being Tested

The performance test runs the actual consumer queries from `internal/data/resource_repository.go`:

### Query 1: FindLatestRepresentations
```sql
SELECT cr.data, cr.version
FROM reporter_resources rr
JOIN common_representations cr ON rr.resource_id = cr.resource_id
WHERE rr.local_resource_id = ? 
  AND rr.reporter_type = ?
  AND rr.resource_type = ?
  AND rr.reporter_instance_id = ?
ORDER BY cr.version DESC
LIMIT 1
```

### Query 2: FindCurrentAndPreviousVersionedRepresentations
```sql
SELECT cr.data, cr.version, cr.resource_id, 
       cr.reported_by_reporter_type, cr.reported_by_reporter_instance, cr.transaction_id
FROM reporter_resources rr
JOIN common_representations cr ON rr.resource_id = cr.resource_id
WHERE rr.local_resource_id = ?
  AND rr.reporter_type = ?
  AND rr.resource_type = ?
  AND rr.reporter_instance_id = ?
  AND (cr.version = ? OR cr.version = ?)
```

## Prerequisites

- Access to an ephemeral environment
- `oc` CLI configured
- Namespace with enough resources (20GB+ disk, 4GB+ RAM recommended)
- 4-6 hours for complete test run

## Running the Test

### Option 1: Automated Test (Recommended)

Use the orchestration script that runs everything automatically:

```bash
cd /Users/snehagunta/git/kessel/inventory-api

# Run complete test suite
./scripts/run-retention-performance-test.sh <namespace>

# Example:
./scripts/run-retention-performance-test.sh ephemeral-abc123

# With custom results directory:
./scripts/run-retention-performance-test.sh ephemeral-abc123 /tmp/my-test-results
```

The script will:
1. Generate 2M resources with ~10M representations (~4 hours)
2. Run baseline performance test (~10 minutes)
3. Run Phase 1 cleanup (~1-2 hours)
4. Test performance after Phase 1
5. Run Phase 2 cleanup (~15 minutes)
6. Test performance after Phase 2
7. Run Phase 3 cleanup (~1-2 hours)
8. Test final performance
9. Generate comprehensive report

**Total time: ~6-8 hours**

### Option 2: Manual Step-by-Step

For more control or troubleshooting:

#### Step 1: Generate Test Data

```bash
oc process -f deploy/kessel-inventory-onetime-job.yaml \
  -p JOB_NAME="test-data-gen-$(date +%Y%m%d%H%M)" \
  -p JOB_COMMAND="test-data-generator-job" \
  -p RESOURCES="2000000" \
  -p VERSIONS_PER_RESOURCE="5" \
  -p OLD_DATA_DAYS="60" \
  -n <namespace> \
| oc apply -f -

# Monitor progress
oc logs -f job/test-data-gen-<timestamp> -n <namespace>
```

#### Step 2: Baseline Performance Test

```bash
oc process -f deploy/kessel-inventory-onetime-job.yaml \
  -p JOB_NAME="perf-baseline-$(date +%Y%m%d%H%M)" \
  -p JOB_COMMAND="performance-test-job" \
  -p QUERY_COUNT="200000" \
  -p CONCURRENCY="50" \
  -p TEST_TYPE="both" \
  -n <namespace> \
| oc apply -f -

# View results
oc logs job/perf-baseline-<timestamp> -n <namespace>
```

#### Step 3: Run Phase 1 Cleanup

```bash
oc process -f deploy/kessel-inventory-onetime-job.yaml \
  -p JOB_NAME="phase1-cleanup-$(date +%Y%m%d%H%M)" \
  -p JOB_COMMAND="reporter-representations-cleanup-job" \
  -p RETENTION_DAYS="7" \
  -p TOMBSTONE_DAYS="30" \
  -p DRY_RUN="false" \
  -n <namespace> \
| oc apply -f -
```

#### Step 4: Test After Phase 1

```bash
oc process -f deploy/kessel-inventory-onetime-job.yaml \
  -p JOB_NAME="perf-phase1-$(date +%Y%m%d%H%M)" \
  -p JOB_COMMAND="performance-test-job" \
  -p QUERY_COUNT="200000" \
  -p CONCURRENCY="50" \
  -n <namespace> \
| oc apply -f -
```

Repeat for Phase 2 and Phase 3.

## Understanding Results

### Performance Metrics

The performance test reports these key metrics:

```
=== FindLatestRepresentations Results ===
Total queries: 200000
Successful: 199998
Failed: 2
Duration: 5m30s
QPS: 606 queries/sec
p50 latency: 45ms
p95 latency: 120ms
p99 latency: 180ms
Min latency: 12ms
Max latency: 850ms
```

**Key metrics to track:**
- **p99 latency** - 99th percentile (worst 1% of queries)
- **QPS** - Queries per second (throughput)
- **Failed queries** - Should be minimal (<0.1%)

### Expected Results

Based on production data (34M resources, 211M representations):

#### Before Cleanup
- **Table sizes:** 91 GB total (68 GB representations, 23 GB resources)
- **p99 latency:** 150-250ms (with large tables)
- **QPS:** 400-600 queries/sec

#### After Phase 1 (Reporter Representations Cleanup)
- **Deleted:** ~70M reporter_representations
- **Table size:** ~50 GB total (-45%)
- **Expected improvement:** 10-15% latency reduction
- **p99 latency:** 130-220ms

#### After Phase 2 (Tombstoned Resources Cleanup)
- **Deleted:** ~3-4M resources completely
- **Table size:** ~45 GB total (-50%)
- **Expected improvement:** Minimal (mostly cleanup of already-excluded data)
- **p99 latency:** 125-215ms

#### After Phase 3 (Common Representations Cleanup)
- **Deleted:** ~70M common_representations
- **Table size:** ~35-40 GB total (-56%)
- **Expected improvement:** 20-30% latency reduction
- **p99 latency:** 90-150ms

**Total expected improvement: 30-40% latency reduction**

### Table Statistics

Query to get table stats before/after each phase:

```sql
SELECT
  c.relname as table_name,
  c.reltuples::bigint as estimated_rows,
  pg_size_pretty(pg_total_relation_size(c.oid)) as total_size,
  pg_size_pretty(pg_relation_size(c.oid)) as table_size,
  pg_size_pretty(pg_total_relation_size(c.oid) - pg_relation_size(c.oid)) as indexes_size
FROM pg_class c
JOIN pg_namespace n ON n.oid = c.relnamespace
WHERE c.relname IN ('resource', 'reporter_resources', 
                    'reporter_representations', 'common_representations')
  AND n.nspname = 'public'
ORDER BY c.reltuples DESC;
```

## Analyzing the Report

The automated script generates a comprehensive report at:
`$RESULTS_DIR/test-report.md`

The report includes:

### 1. Table Statistics at Each Phase
- Row counts before/after each cleanup
- Disk space savings
- Index sizes

### 2. Performance Test Results
- Baseline (before cleanup)
- After Phase 1
- After Phase 2  
- Final (after Phase 3)

### 3. Cleanup Job Execution Times
- Time taken by each cleanup phase
- Rows deleted per phase

### 4. Key Findings Section
Fill this in after reviewing results:

```markdown
## Key Findings

1. **Database Size Impact**
   - Baseline: 91 GB, p99 latency: 180ms
   - Final: 40 GB, p99 latency: 110ms
   - **Conclusion:** 56% size reduction → 39% latency improvement

2. **Phase-by-Phase Impact**
   - Phase 1 (reporter_reps): 15% latency improvement
   - Phase 2 (tombstoned): Minimal impact (5%)
   - Phase 3 (common_reps): 25% latency improvement
   - **Conclusion:** common_representations cleanup has biggest impact

3. **Cleanup Efficiency**
   - Phase 1: Deleted 70M rows in 1.5 hours (13K rows/sec)
   - Phase 2: Deleted 200K resources in 15 minutes
   - Phase 3: Deleted 70M rows in 1.5 hours (13K rows/sec)
   - **Conclusion:** Cleanup is efficient and predictable
```

## Troubleshooting

### Test Data Generation is Slow

Expected: ~163 resources/sec (~3-4 hours for 2M resources)

If slower:
- Check database pod resources
- Reduce batch size: `--batch-size=500`
- Reduce version count: `--versions-per-resource=3`

### Performance Test Shows No Improvement

Possible causes:
1. **Indexes not created** - Check if created_at indexes exist
2. **VACUUM not run** - PostgreSQL may not have reclaimed space yet
3. **Wrong queries** - Verify test is running actual consumer queries
4. **Query plan changed** - Run `EXPLAIN ANALYZE` on queries

### Cleanup Jobs Timeout

Increase timeout in job spec or reduce batch size:
```yaml
spec:
  activeDeadlineSeconds: 28800  # 8 hours
```

### High Query Failure Rate

If >1% queries fail:
- Check database connectivity
- Check for locks or long-running transactions
- Review error logs

## Performance Test Configuration

### Default Configuration

```bash
QUERY_COUNT=200000        # Total queries to run
CONCURRENCY=50            # Concurrent workers
TEST_TYPE=both            # Test both query types
```

### Custom Configuration

For faster test (less accurate):
```bash
--query-count=50000 --concurrency=25
```

For more thorough test (slower):
```bash
--query-count=500000 --concurrency=100
```

For specific query type only:
```bash
--test-type=latest          # Only FindLatestRepresentations
--test-type=versioned       # Only FindCurrentAndPreviousVersionedRepresentations
```

## Repeating the Test

To run the test multiple times:

1. **Clean up previous test data:**
```bash
# Connect to database and truncate tables
oc rsh <db-pod> psql -c "TRUNCATE TABLE reporter_representations CASCADE;"
```

2. **Run test with different parameters:**
```bash
# Test with different retention periods
./scripts/run-retention-performance-test.sh <namespace> /tmp/test-retention-14days
# Then modify script to use RETENTION_DAYS=14
```

3. **Compare results across test runs**

## CI/CD Integration

To automate this test in CI:

```yaml
- name: Retention Performance Test
  run: |
    ./scripts/run-retention-performance-test.sh $EPHEMERAL_NAMESPACE $RESULTS_DIR
    # Upload results
    gh issue comment $PR_NUMBER --body-file $RESULTS_DIR/test-report.md
```

## Next Steps

After running the test:

1. **Review the report** - Analyze key findings
2. **Share with team** - Post results to Slack/Jira
3. **Make decision** - Proceed with production rollout based on results
4. **Document findings** - Update this guide with actual results

## Example Complete Report

See `/tmp/retention-perf-test-<timestamp>/test-report.md` for an example report after running the automated test.
