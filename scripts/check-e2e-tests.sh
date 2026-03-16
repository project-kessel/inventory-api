#!/bin/bash
set -ex

EXISTING_CLUSTER=$(kind get clusters 2> /dev/null | grep inventory-cluster)
if [[ -z "$EXISTING_CLUSTER" ]]; then
  echo "kind cluster 'inventory-cluster' not found (hint: try 'make inventory-up-kind')"
  exit 1
fi

# check_job polls a Kubernetes Job by selector until it completes or fails.
# Remove the e2e-inventory-outbox-table-tests call below when the outbox table is deprecated.
check_job() {
  local job_name=$1
  local selector="job-name=${job_name}"

  echo "=== Checking ${job_name} ==="
  TEST_POD=$(kubectl get pods --selector=${selector} -o jsonpath='{.items[0].metadata.name}')
  for i in {1..50}; do
    STATUS=$(kubectl get pods --selector=${selector} -o jsonpath='{.items[0].status.containerStatuses[0].state.terminated.reason}')
    if [ "$STATUS" = "Completed" ]; then
      echo "${job_name} completed successfully."
      kubectl logs $TEST_POD
      return 0
    elif [ "$STATUS" = "Error" ]; then
      echo "${job_name} failed."
      kubectl logs $TEST_POD
      return 1
    fi
    sleep 3
  done
  kubectl logs $TEST_POD
  echo "Unexpected timeout, ${job_name} did not complete"
  return 1
}

# WAL-mode tests are verified inline by start-inventory-kind.sh before the
# table-mode run begins, so we only need to check the table-mode job here.
check_job "e2e-inventory-outbox-table-tests"

kubectl get pods
kubectl get svc
