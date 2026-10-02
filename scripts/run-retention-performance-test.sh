#!/bin/bash
set -euo pipefail

# Retention Cleanup Performance Test Orchestrator
# This script runs a comprehensive end-to-end performance test:
# 1. Generate test data
# 2. Run baseline performance test
# 3. Run Phase 1 cleanup (reporter_representations)
# 4. Run performance test
# 5. Run Phase 2 cleanup (tombstoned resources)
# 6. Run performance test
# 7. Run Phase 3 cleanup (common_representations)
# 8. Run final performance test
# 9. Generate detailed report

NAMESPACE=${1:-""}
RESULTS_DIR=${2:-"/tmp/retention-perf-test-$(date +%Y%m%d-%H%M%S)"}

if [ -z "$NAMESPACE" ]; then
  echo "Usage: $0 <namespace> [results-dir]"
  echo "Example: $0 ephemeral-abc123 /tmp/my-test-results"
  exit 1
fi

echo "=================================================="
echo "Retention Cleanup Performance Test"
echo "Namespace: $NAMESPACE"
echo "Results Directory: $RESULTS_DIR"
echo "=================================================="

mkdir -p "$RESULTS_DIR"

# Configuration
RESOURCES=2000000
VERSIONS_PER_RESOURCE=5
OLD_DATA_DAYS=60
TOMBSTONE_PERCENT=20
OLD_TOMBSTONE_PERCENT=50
QUERY_COUNT=200000
CONCURRENCY=50

# Helper function to run a job and wait for completion
run_job() {
  local job_name=$1
  local job_command=$2
  shift 2
  local extra_args="$@"

  echo ""
  echo "===> Running job: $job_name"
  echo "Command: $job_command"
  echo "Args: $extra_args"

  oc process -f deploy/kessel-inventory-onetime-job.yaml \
    -p JOB_NAME="$job_name" \
    -p JOB_COMMAND="$job_command" \
    $extra_args \
    -n "$NAMESPACE" \
  | oc apply -f - -n "$NAMESPACE"

  # Wait for job to complete
  echo "Waiting for job to complete..."
  oc wait --for=condition=complete --timeout=4h job/"$job_name" -n "$NAMESPACE" || {
    echo "Job failed or timed out"
    oc logs job/"$job_name" -n "$NAMESPACE" | tail -100
    return 1
  }

  # Get logs
  local log_file="$RESULTS_DIR/${job_name}.log"
  oc logs job/"$job_name" -n "$NAMESPACE" > "$log_file"
  echo "Logs saved to: $log_file"

  # Clean up job
  oc delete job/"$job_name" -n "$NAMESPACE" || true

  echo "===> Job completed: $job_name"
}

# Function to get table statistics
get_table_stats() {
  local phase=$1
  local output_file="$RESULTS_DIR/table-stats-${phase}.json"

  echo ""
  echo "===> Getting table statistics for phase: $phase"

  # Port forward to database
  POD=$(oc get pods -n "$NAMESPACE" -l app=kessel-inventory-db -o jsonpath='{.items[0].metadata.name}')
  oc port-forward "$POD" 15432:5432 -n "$NAMESPACE" &
  PF_PID=$!
  sleep 5

  # Get statistics
  PGPASSWORD=$(oc get secret kessel-inventory-db -n "$NAMESPACE" -o jsonpath='{.data.db\.password}' | base64 -d) \
  psql -h localhost -p 15432 -U $(oc get secret kessel-inventory-db -n "$NAMESPACE" -o jsonpath='{.data.db\.user}' | base64 -d) \
    -d $(oc get secret kessel-inventory-db -n "$NAMESPACE" -o jsonpath='{.data.db\.name}' | base64 -d) \
    -t -A -F"," -c "
      SELECT
        row_to_json(t)
      FROM (
        SELECT
          c.relname as table_name,
          c.reltuples::bigint as estimated_rows,
          pg_size_pretty(pg_total_relation_size(c.oid)) as total_size,
          pg_size_pretty(pg_relation_size(c.oid)) as table_size,
          pg_size_pretty(pg_total_relation_size(c.oid) - pg_relation_size(c.oid)) as indexes_size,
          '$phase' as phase,
          now() as timestamp
        FROM pg_class c
        JOIN pg_namespace n ON n.oid = c.relnamespace
        WHERE c.relname IN ('resource', 'reporter_resources', 'reporter_representations', 'common_representations')
          AND n.nspname = 'public'
        ORDER BY c.reltuples DESC
      ) t;
    " > "$output_file"

  kill $PF_PID 2>/dev/null || true
  wait $PF_PID 2>/dev/null || true

  echo "Table stats saved to: $output_file"
}

# Main test flow
echo ""
echo "=================================================="
echo "STEP 1: Generate Test Data"
echo "=================================================="

get_table_stats "00-before-data-generation"

run_job "test-data-gen-$(date +%Y%m%d%H%M)" \
  "test-data-generator-job" \
  -p RESOURCES="$RESOURCES" \
  -p VERSIONS_PER_RESOURCE="$VERSIONS_PER_RESOURCE" \
  -p OLD_DATA_DAYS="$OLD_DATA_DAYS" \
  -p TOMBSTONE_PERCENT="$TOMBSTONE_PERCENT" \
  -p OLD_TOMBSTONE_PERCENT="$OLD_TOMBSTONE_PERCENT"

get_table_stats "01-after-data-generation"

echo ""
echo "=================================================="
echo "STEP 2: Baseline Performance Test"
echo "=================================================="

run_job "perf-test-baseline-$(date +%Y%m%d%H%M)" \
  "performance-test-job" \
  -p QUERY_COUNT="$QUERY_COUNT" \
  -p CONCURRENCY="$CONCURRENCY" \
  -p TEST_TYPE="both"

echo ""
echo "=================================================="
echo "STEP 3: Phase 1 Cleanup (reporter_representations)"
echo "=================================================="

run_job "phase1-cleanup-$(date +%Y%m%d%H%M)" \
  "reporter-representations-cleanup-job" \
  -p RETENTION_DAYS="7" \
  -p TOMBSTONE_DAYS="30" \
  -p DRY_RUN="false"

get_table_stats "02-after-phase1-cleanup"

run_job "perf-test-phase1-$(date +%Y%m%d%H%M)" \
  "performance-test-job" \
  -p QUERY_COUNT="$QUERY_COUNT" \
  -p CONCURRENCY="$CONCURRENCY" \
  -p TEST_TYPE="both"

echo ""
echo "=================================================="
echo "STEP 4: Phase 2 Cleanup (tombstoned resources)"
echo "=================================================="

run_job "phase2-cleanup-$(date +%Y%m%d%H%M)" \
  "tombstoned-resources-cleanup-job" \
  -p TOMBSTONE_DAYS="30" \
  -p DRY_RUN="false"

get_table_stats "03-after-phase2-cleanup"

run_job "perf-test-phase2-$(date +%Y%m%d%H%M)" \
  "performance-test-job" \
  -p QUERY_COUNT="$QUERY_COUNT" \
  -p CONCURRENCY="$CONCURRENCY" \
  -p TEST_TYPE="both"

echo ""
echo "=================================================="
echo "STEP 5: Phase 3 Cleanup (common_representations)"
echo "=================================================="

run_job "phase3-cleanup-$(date +%Y%m%d%H%M)" \
  "common-representations-cleanup-job" \
  -p RETENTION_DAYS="7" \
  -p TOMBSTONE_DAYS="30" \
  -p DRY_RUN="false"

get_table_stats "04-after-phase3-cleanup"

run_job "perf-test-final-$(date +%Y%m%d%H%M)" \
  "performance-test-job" \
  -p QUERY_COUNT="$QUERY_COUNT" \
  -p CONCURRENCY="$CONCURRENCY" \
  -p TEST_TYPE="both"

echo ""
echo "=================================================="
echo "STEP 6: Generate Test Report"
echo "=================================================="

# Generate comprehensive report
cat > "$RESULTS_DIR/test-report.md" << EOF
# Retention Cleanup Performance Test Report

**Test Date:** $(date)
**Namespace:** $NAMESPACE
**Test Configuration:**
- Resources Generated: $RESOURCES
- Versions per Resource: $VERSIONS_PER_RESOURCE
- Old Data Days: $OLD_DATA_DAYS
- Tombstone Percent: $TOMBSTONE_PERCENT%
- Old Tombstone Percent: $OLD_TOMBSTONE_PERCENT%
- Query Count per Test: $QUERY_COUNT
- Concurrency: $CONCURRENCY workers

## Test Results

### Table Statistics

#### Before Test
\`\`\`
$(cat "$RESULTS_DIR/table-stats-00-before-data-generation.json")
\`\`\`

#### After Data Generation
\`\`\`
$(cat "$RESULTS_DIR/table-stats-01-after-data-generation.json")
\`\`\`

#### After Phase 1 (Reporter Representations Cleanup)
\`\`\`
$(cat "$RESULTS_DIR/table-stats-02-after-phase1-cleanup.json")
\`\`\`

#### After Phase 2 (Tombstoned Resources Cleanup)
\`\`\`
$(cat "$RESULTS_DIR/table-stats-03-after-phase2-cleanup.json")
\`\`\`

#### After Phase 3 (Common Representations Cleanup)
\`\`\`
$(cat "$RESULTS_DIR/table-stats-04-after-phase3-cleanup.json")
\`\`\`

### Performance Test Results

#### Baseline (Before Cleanup)
\`\`\`
$(grep "=== .*Results ===" "$RESULTS_DIR/perf-test-baseline-"*.log -A 10)
\`\`\`

#### After Phase 1
\`\`\`
$(grep "=== .*Results ===" "$RESULTS_DIR/perf-test-phase1-"*.log -A 10)
\`\`\`

#### After Phase 2
\`\`\`
$(grep "=== .*Results ===" "$RESULTS_DIR/perf-test-phase2-"*.log -A 10)
\`\`\`

#### Final (After Phase 3)
\`\`\`
$(grep "=== .*Results ===" "$RESULTS_DIR/perf-test-final-"*.log -A 10)
\`\`\`

### Cleanup Job Execution Times

#### Phase 1: Reporter Representations Cleanup
\`\`\`
$(grep -E "Starting|completed" "$RESULTS_DIR/phase1-cleanup-"*.log)
\`\`\`

#### Phase 2: Tombstoned Resources Cleanup
\`\`\`
$(grep -E "Starting|completed" "$RESULTS_DIR/phase2-cleanup-"*.log)
\`\`\`

#### Phase 3: Common Representations Cleanup
\`\`\`
$(grep -E "Starting|completed" "$RESULTS_DIR/phase3-cleanup-"*.log)
\`\`\`

## Analysis

### Key Findings

1. **Database Size Impact**
   - Baseline table sizes vs final table sizes
   - Percentage reduction in each phase

2. **Query Performance Impact**
   - p99 latency improvements across phases
   - QPS (queries per second) improvements

3. **Cleanup Efficiency**
   - Time taken by each cleanup phase
   - Rows deleted per phase

### Conclusions

[To be filled after reviewing results]

## All Logs

All detailed logs are available in: \`$RESULTS_DIR/\`

EOF

echo ""
echo "=================================================="
echo "Test Complete!"
echo "=================================================="
echo "Results directory: $RESULTS_DIR"
echo "Test report: $RESULTS_DIR/test-report.md"
echo ""
echo "Next steps:"
echo "1. Review the test report"
echo "2. Analyze performance improvements"
echo "3. Share results with the team"
echo ""
