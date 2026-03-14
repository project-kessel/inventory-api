#!/bin/bash
set -e

source ./scripts/check_docker_podman.sh

check_kafka_readiness() {
  local pod_name=$1
  local max_retries=$2
  local retry_count=0

  echo "Waiting for pod $pod_name readiness..."

  while true; do
    kubectl exec $pod_name -- /opt/kafka/kafka_readiness.sh >/dev/null 2>&1
    if [[ $? -eq 0 ]]; then
      echo "Pod $pod_name is ready."
      break
    else
      echo "Pod $pod_name is not ready yet. Retrying in 10 seconds..."
      sleep 15
      ((retry_count++))
      if [[ $retry_count -ge $max_retries ]]; then
        echo "Timeout waiting for pod $pod_name readiness."
        echo "Logs from pod $pod_name:"
        kubectl logs $pod_name
        echo "Describing pod $pod_name:"
        kubectl describe pod $pod_name
        rm -rf $TMP_DIR
        rm -rf $KIND
        exit 1
      fi
    fi
  done
}

# default kind in github ubuntu running is not playing well with strimzi operator
# purposefully installing an older version (0.27.0) to get tests back up and running till
# and alternate solution is found

TMP_DIR=$(mktemp -d)
KIND=${TMP_DIR}/kind27
wget -O $KIND https://github.com/kubernetes-sigs/kind/releases/download/v0.27.0/kind-linux-amd64
chmod +x $KIND


# check for existing cluster to add some idempotency when testing locally
# the existing cluster check exits with 1 if none are found, temp turn off auto exit with `set`
set +e
EXISTING_CLUSTER=$($KIND get clusters 2> /dev/null | grep inventory-cluster)
set -e
if [[ -z "$EXISTING_CLUSTER" ]]; then $KIND create cluster --name inventory-cluster; fi

# build/tag image
${DOCKER} build -t localhost/inventory-api:latest -f Dockerfile .
${DOCKER} build -t localhost/inventory-e2e-tests:latest -f Dockerfile-e2e .
${DOCKER} build -t localhost/kafka-connect:latest -f Dockerfile.connect .

${DOCKER} tag localhost/inventory-api:latest localhost/inventory-api:e2e-test
${DOCKER} tag localhost/inventory-e2e-tests:latest localhost/inventory-e2e-tests:e2e-test
${DOCKER} tag localhost/kafka-connect:latest localhost/kafka-connect:e2e-test

rm -rf inventory-api.tar
rm -rf inventory-e2e-tests.tar
rm -rf kafka-connect.tar

${DOCKER} save -o inventory-api.tar localhost/inventory-api:e2e-test
${DOCKER} save -o inventory-e2e-tests.tar localhost/inventory-e2e-tests:e2e-test
${DOCKER} save -o kafka-connect.tar localhost/kafka-connect:e2e-test

$KIND load image-archive inventory-api.tar --name inventory-cluster
$KIND load image-archive inventory-e2e-tests.tar --name inventory-cluster
$KIND load image-archive kafka-connect.tar --name inventory-cluster

[ -f resources.tar.gz ] || tar czf resources.tar.gz -C data/schema/resources .
kubectl get configmap resources-tarball || kubectl create configmap resources-tarball --from-file=resources.tar.gz

kubectl apply -f deploy/kind/strimzi-operator/strimzi-cluster-operator-0.45.0.yaml
kubectl apply -f deploy/kind/inventory/kessel-inventory.yaml
kubectl apply -f deploy/kind/inventory/invdatabase.yaml
kubectl apply -f deploy/kind/inventory/strimzi.yaml


kubectl apply -f https://projectcontour.io/quickstart/contour.yaml
kubectl get crd httpproxies.projectcontour.io

# checks for the config map first, or creates it if not found
kubectl get configmap spicedb-schema || kubectl create configmap spicedb-schema --from-file=deploy/schema.zed

kubectl apply -f deploy/kind/relations/spicedb-kind-setup/postgres/secret.yaml
kubectl apply -f deploy/kind/relations/spicedb-kind-setup/postgres/postgresql.yaml
kubectl apply -f deploy/kind/relations/spicedb-kind-setup/postgres/storage.yaml

# Install SpiceDB operator if not already installed
kubectl get crd spicedbclusters.authzed.com > /dev/null 2>&1 || \
  kubectl apply --server-side -f https://github.com/authzed/spicedb-operator/releases/download/v1.21.0/bundle.yaml

kubectl apply -f deploy/kind/relations/spicedb-kind-setup/spicedb-cr.yaml
kubectl apply -f deploy/kind/relations/spicedb-kind-setup/svc-ingress.yaml
kubectl apply -f deploy/kind/relations/spicedb-kind-setup/relations-api/secret.yaml
kubectl apply -f deploy/kind/relations/spicedb-kind-setup/relations-api/deployment.yaml
kubectl apply -f deploy/kind/relations/spicedb-kind-setup/relations-api/svc.yaml


echo "Waiting for all pods to be ready (1/1)..."
MAX_RETRIES=30


while true; do
  POD_STATUSES=$(kubectl get pods --no-headers)

  NOT_READY=$(echo "$POD_STATUSES" | awk '{print $2}' | grep -v '^1/1$' | wc -l)

  if [ "$NOT_READY" -eq 0 ]; then
    echo "All pods are ready (1/1)."
    echo "Delaying readiness checks to allow Kafka pods to initialize..."
    sleep 30
    check_kafka_readiness "my-cluster-kafka-0" $MAX_RETRIES
    break
  fi

  echo "Waiting for pods to be ready... ($NOT_READY pods not ready)"
  kubectl get pods
  sleep 5

done

# --- Run WAL-mode e2e tests ---
echo "=== Running e2e tests with outbox-mode=wal ==="
kubectl apply -f deploy/kind/e2e/e2e-batch.yaml

echo "Waiting for WAL e2e tests to complete..."
WAL_RETRIES=50
for i in $(seq 1 $WAL_RETRIES); do
  STATUS=$(kubectl get pods --selector=job-name=e2e-inventory-http-tests -o jsonpath='{.items[0].status.containerStatuses[0].state.terminated.reason}' 2>/dev/null)
  if [ "$STATUS" = "Completed" ]; then
    echo "WAL e2e tests passed."
    kubectl logs $(kubectl get pods --selector=job-name=e2e-inventory-http-tests -o jsonpath='{.items[0].metadata.name}')
    break
  elif [ "$STATUS" = "Error" ]; then
    echo "WAL e2e tests failed."
    kubectl logs $(kubectl get pods --selector=job-name=e2e-inventory-http-tests -o jsonpath='{.items[0].metadata.name}')
    rm -rf $TMP_DIR
    exit 1
  fi
  sleep 3
done
if [ "$STATUS" != "Completed" ]; then
  echo "WAL e2e tests timed out."
  kubectl logs $(kubectl get pods --selector=job-name=e2e-inventory-http-tests -o jsonpath='{.items[0].metadata.name}') 2>/dev/null || true
  rm -rf $TMP_DIR
  exit 1
fi

# --- Swap to table-mode and run e2e tests ---
# Remove this block when the outbox table is deprecated.
echo "=== Switching to outbox-mode=table for second e2e run ==="

# Patch inventory config to use table mode
kubectl get secret inventory-api-config -o jsonpath='{.data.inventory-api-config\.yaml}' | base64 -d | sed 's/outbox-mode: wal/outbox-mode: table/' > /tmp/inventory-api-config-table.yaml
kubectl create secret generic inventory-api-config \
  --from-file=inventory-api-config.yaml=/tmp/inventory-api-config-table.yaml \
  --from-literal=db_password=yPsw5e6ab4bvAGe5H \
  --dry-run=client -o yaml | kubectl apply -f -
rm -f /tmp/inventory-api-config-table.yaml

# Swap connector from WAL to table mode — wait for slot cleanup before recreating
kubectl delete kafkaconnector kessel-inventory-source-connector
sleep 10
kubectl apply -f deploy/kind/inventory/strimzi-table-connector.yaml

# Restart inventory pod to pick up new config
kubectl rollout restart deployment kessel-inventory
kubectl rollout status deployment kessel-inventory --timeout=120s

echo "Waiting for inventory service to be ready after restart..."
until kubectl exec $(kubectl get pods -l app=kessel-inventory -o jsonpath='{.items[0].metadata.name}') -- curl -sf http://localhost:8081/api/kessel/v1/readyz 2>/dev/null; do
  sleep 5
done

# Submit table-mode e2e tests
kubectl apply -f deploy/kind/e2e/e2e-batch-outbox-table.yaml

echo "Setup complete."
rm -rf $TMP_DIR
rm -rf $KIND
