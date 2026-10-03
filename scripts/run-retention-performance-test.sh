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

  # Get the inventory-api image
  local image=$(oc get deployment kessel-inventory-api -n "$NAMESPACE" -o jsonpath="{.spec.template.spec.containers[0].image}")

  # Format extra args as YAML list items
  local formatted_args=""
  if [ -n "$extra_args" ]; then
    for arg in $extra_args; do
      formatted_args="${formatted_args}        - $arg"$"\n"
    done
  fi

  # Create job from template
