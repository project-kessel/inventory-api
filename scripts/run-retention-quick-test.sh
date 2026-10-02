#!/bin/bash
set -euo pipefail

# Quick Retention Cleanup Performance Test
# Runs a faster version of the full test for validation/iteration
# Time: ~30-60 minutes instead of 6-8 hours

NAMESPACE=${1:-""}
RESULTS_DIR=${2:-"/tmp/retention-quick-test-$(date +%Y%m%d-%H%M%S)"}

if [ -z "$NAMESPACE" ]; then
  echo "Usage: $0 <namespace> [results-dir]"
  echo "Example: $0 ephemeral-abc123"
  exit 1
fi

echo "=================================================="
echo "Quick Retention Cleanup Performance Test"
echo "Namespace: $NAMESPACE"
echo "Results Directory: $RESULTS_DIR"
echo "Estimated time: 30-60 minutes"
echo "=================================================="

mkdir -p "$RESULTS_DIR"

# Quick test configuration (much smaller than full test)
RESOURCES=100000              # 100K instead of 2M
VERSIONS_PER_RESOURCE=3       # 3 instead of 5
OLD_DATA_DAYS=30              # 30 instead of 60
TOMBSTONE_PERCENT=20
OLD_TOMBSTONE_PERCENT=50
QUERY_COUNT=10000             # 10K instead of 200K
CONCURRENCY=25                # 25 instead of 50

echo ""
echo "Test Configuration:"
echo "  Resources: $RESOURCES"
echo "  Versions per resource: $VERSIONS_PER_RESOURCE"
echo "  Total representations: ~$((RESOURCES * VERSIONS_PER_RESOURCE))"
echo "  Query count: $QUERY_COUNT"
echo "  Concurrency: $CONCURRENCY workers"
echo ""

# Helper function to run a job and wait for completion
run_job() {
  local job_name=$1
  local job_command=$2
  shift 2
  local extra_args="$@"

  echo ""
  echo "===> Running job: $job_name"

  # Create job from template
  cat <<EOF | oc apply -f - -n "$NAMESPACE"
apiVersion: batch/v1
kind: Job
metadata:
  name: $job_name
spec:
  template:
    spec:
      containers:
      - name: inventory-job
        image: \$(oc get deployment kessel-inventory-api -n $NAMESPACE -o jsonpath='{.spec.template.spec.containers[0].image}')
        command:
        - /usr/local/bin/inventory-api
        - run-job
        - $job_command
        $(echo "$extra_args" | sed 's/^/        - /')
        - --storage.database=postgres
        - --storage.postgres.host=\$(DB_HOST)
        - --storage.postgres.dbname=\$(DB_NAME)
        - --storage.postgres.user=\$(DB_USER)
        - --storage.postgres.password=\$(DB_PASSWORD)
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
EOF

  # Wait for job to complete
  echo "Waiting for job to complete..."
  oc wait --for=condition=complete --timeout=2h job/"$job_name" -n "$NAMESPACE" || {
    echo "Job failed or timed out"
    oc logs job/"$job_name" -n "$NAMESPACE" --tail=100
    return 1
  }

  # Get logs
  local log_file="$RESULTS_DIR/${job_name}.log"
  oc logs job/"$job_name" -n "$NAMESPACE" > "$log_file"
  echo "Logs saved to: $log_file"

  # Extract key metrics from logs
  echo "Key metrics:"
  grep -E "p99 latency|QPS|Deleted|completed" "$log_file" | tail -10 || true

  # Clean up job
  oc delete job/"$job_name" -n "$NAMESPACE" || true

  echo "===> Job completed: $job_name"
}

# Function to get table statistics
get_table_stats() {
  local phase=$1
  local output_file="$RESULTS_DIR/table-stats-${phase}.txt"

  echo ""
  echo "===> Getting table statistics for phase: $phase"

  # Get stats using oc exec
  POD=$(oc get pods -n "$NAMESPACE" -l app=kessel-inventory-db -o jsonpath='{.items[0].metadata.name}')

  oc exec "$POD" -n "$NAMESPACE" -- bash -c "
    PGPASSWORD=\$(cat /run/secrets/db.password) psql -h localhost -U \$(cat /run/secrets/db.user) -d \$(cat /run/secrets/db.name) -t -A -F',' -c \"
      SELECT
        c.relname as table_name,
        c.reltuples::bigint as estimated_rows,
        pg_size_pretty(pg_total_relation_size(c.oid)) as total_size
      FROM pg_class c
      JOIN pg_namespace n ON n.oid = c.relnamespace
      WHERE c.relname IN ('resource', 'reporter_resources', 'reporter_representations', 'common_representations')
        AND n.nspname = 'public'
      ORDER BY c.reltuples DESC;
    \"
  " > "$output_file"

  echo "Table stats:"
  column -t -s',' "$output_file"
  echo ""
}

# Main test flow
echo ""
echo "=================================================="
echo "STEP 1: Generate Test Data (~10-15 minutes)"
echo "=================================================="

get_table_stats "00-before-data"

run_job "test-data-$(date +%Y%m%d%H%M)" \
  "test-data-generator-job" \
  "--resources=$RESOURCES" \
  "--versions-per-resource=$VERSIONS_PER_RESOURCE" \
  "--old-data-days=$OLD_DATA_DAYS" \
  "--tombstone-percent=$TOMBSTONE_PERCENT" \
  "--old-tombstone-percent=$OLD_TOMBSTONE_PERCENT"

get_table_stats "01-after-data"

echo ""
echo "=================================================="
echo "STEP 2: Baseline Performance Test (~2 minutes)"
echo "=================================================="

run_job "perf-baseline-$(date +%Y%m%d%H%M)" \
  "performance-test-job" \
  "--query-count=$QUERY_COUNT" \
  "--concurrency=$CONCURRENCY" \
  "--test-type=both"

echo ""
echo "=================================================="
echo "STEP 3: Phase 1 Cleanup (~5 minutes)"
echo "=================================================="

run_job "phase1-$(date +%Y%m%d%H%M)" \
  "reporter-representations-cleanup-job" \
  "--retention-days=7" \
  "--tombstone-days=30"

get_table_stats "02-after-phase1"

run_job "perf-phase1-$(date +%Y%m%d%H%M)" \
  "performance-test-job" \
  "--query-count=$QUERY_COUNT" \
  "--concurrency=$CONCURRENCY" \
  "--test-type=both"

echo ""
echo "=================================================="
echo "STEP 4: Phase 2 Cleanup (~2 minutes)"
echo "=================================================="

run_job "phase2-$(date +%Y%m%d%H%M)" \
  "tombstoned-resources-cleanup-job" \
  "--tombstone-days=30"

get_table_stats "03-after-phase2"

run_job "perf-phase2-$(date +%Y%m%d%H%M)" \
  "performance-test-job" \
  "--query-count=$QUERY_COUNT" \
  "--concurrency=$CONCURRENCY" \
  "--test-type=both"

echo ""
echo "=================================================="
echo "STEP 5: Phase 3 Cleanup (~5 minutes)"
echo "=================================================="

run_job "phase3-$(date +%Y%m%d%H%M)" \
  "common-representations-cleanup-job" \
  "--retention-days=7" \
  "--tombstone-days=30"

get_table_stats "04-after-phase3"

run_job "perf-final-$(date +%Y%m%d%H%M)" \
  "performance-test-job" \
  "--query-count=$QUERY_COUNT" \
  "--concurrency=$CONCURRENCY" \
  "--test-type=both"

echo ""
echo "=================================================="
echo "STEP 6: Generate Quick Report"
echo "=================================================="

# Generate summary report
cat > "$RESULTS_DIR/quick-test-report.md" << 'EOFMD'
# Quick Retention Cleanup Performance Test Report

**Test Date:** $(date)
**Namespace:** $NAMESPACE
**Test Type:** Quick validation test
**Configuration:**
- Resources: $RESOURCES
- Versions per Resource: $VERSIONS_PER_RESOURCE
- Query Count: $QUERY_COUNT
- Concurrency: $CONCURRENCY workers

## Table Statistics

### Before Test
```
$(cat "$RESULTS_DIR/table-stats-00-before-data.txt")
```

### After Data Generation
```
$(cat "$RESULTS_DIR/table-stats-01-after-data.txt")
```

### After Phase 1
```
$(cat "$RESULTS_DIR/table-stats-02-after-phase1.txt")
```

### After Phase 2
```
$(cat "$RESULTS_DIR/table-stats-03-after-phase2.txt")
```

### After Phase 3 (Final)
```
$(cat "$RESULTS_DIR/table-stats-04-after-phase3.txt")
```

## Performance Results

### Baseline
```
$(grep -A 10 "FindLatestRepresentations Results" "$RESULTS_DIR/perf-baseline-"*.log | head -15)
```

### After Phase 1
```
$(grep -A 10 "FindLatestRepresentations Results" "$RESULTS_DIR/perf-phase1-"*.log | head -15)
```

### After Phase 2
```
$(grep -A 10 "FindLatestRepresentations Results" "$RESULTS_DIR/perf-phase2-"*.log | head -15)
```

### Final (After Phase 3)
```
$(grep -A 10 "FindLatestRepresentations Results" "$RESULTS_DIR/perf-final-"*.log | head -15)
```

## Key Findings

[Review the numbers above and document findings]

EOFMD

echo ""
echo "=================================================="
echo "Quick Test Complete!"
echo "=================================================="
echo "Results directory: $RESULTS_DIR"
echo "Quick report: $RESULTS_DIR/quick-test-report.md"
echo ""
echo "Next steps:"
echo "1. Review the quick report"
echo "2. Validate the test framework works"
echo "3. Run full test if results look good"
echo ""
