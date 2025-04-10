#!/bin/bash
set -ex

EXISTING_CLUSTER=$(kind get clusters 2> /dev/null | grep inventory-cluster)
if [[ -z "$EXISTING_CLUSTER" ]]; then
  echo "kind cluster 'inventory-cluster' not found (hint: try 'make inventory-up-kind')"
  exit 1
fi

# View Test Pod Logs
TEST_POD=$(kubectl get pods --selector=job-name=e2e-inventory-http-tests -o jsonpath='{.items[0].metadata.name}')
for i in {1..50}; do
  STATUS=$(kubectl get pods --selector=job-name=e2e-inventory-http-tests -o jsonpath='{.items[0].status.containerStatuses[0].state.terminated.reason}')
  if [ "$STATUS" = "Completed" ]; then
    echo "E2E test pod completed successfully."
    kubectl logs $TEST_POD
    kubectl get pods
    kubectl get svc
    exit 0
  elif [ "$STATUS" = "Error" ]; then
    echo "E2E test pod failed."
    kubectl logs $TEST_POD
    kubectl get pods
    exit 1
  fi
  sleep 3
done
kubectl logs $TEST_POD
echo "Unexpected timeout, test pod did not complete"
exit 1
