#!/bin/bash
set -e

source ./scripts/check_docker_podman.sh

kind create cluster --name inventory-cluster

# build/tag image
${DOCKER} build -t localhost/inventory-api:latest -f Dockerfile .
${DOCKER} build -t localhost/inventory-e2e-tests:latest -f Dockerfile-e2e .

${DOCKER} tag localhost/inventory-api:latest localhost/inventory-api:e2e-test
${DOCKER} tag localhost/inventory-e2e-tests:latest localhost/inventory-e2e-tests:e2e-test

rm -rf inventory-api.tar
rm -rf inventory-e2e-tests.tar

${DOCKER} save -o inventory-api.tar localhost/inventory-api:e2e-test
${DOCKER} save -o inventory-e2e-tests.tar localhost/inventory-e2e-tests:e2e-test

kind load image-archive inventory-api.tar --name inventory-cluster
kind load image-archive inventory-e2e-tests.tar --name inventory-cluster

kubectl create configmap inventory-api-psks --from-file=config/psks.yaml

kubectl apply -f https://strimzi.io/install/latest\?namespace\=default
kubectl apply -f deploy/kind/inventory/kessel-inventory.yaml
kubectl apply -f deploy/kind/inventory/invdatabase.yaml
kubectl apply -f deploy/kind/inventory/strimzi.yaml


kubectl apply -f https://projectcontour.io/quickstart/contour.yaml
kubectl get crd httpproxies.projectcontour.io

# add configmap for spicedb schema
kubectl create configmap spicedb-schema --from-file=deploy/schema.zed

kubectl apply -f deploy/kind/relations/spicedb-kind-setup/postgres/secret.yaml
kubectl apply -f deploy/kind/relations/spicedb-kind-setup/postgres/postgresql.yaml
kubectl apply -f deploy/kind/relations/spicedb-kind-setup/postgres/storage.yaml
kubectl apply -f deploy/kind/relations/spicedb-kind-setup/bundle.yaml
kubectl apply -f deploy/kind/relations/spicedb-kind-setup/spicedb-cr.yaml
kubectl apply -f deploy/kind/relations/spicedb-kind-setup/svc-ingress.yaml
kubectl apply -f deploy/kind/relations/spicedb-kind-setup/relations-api/secret.yaml
kubectl apply -f deploy/kind/relations/spicedb-kind-setup/relations-api/deployment.yaml
kubectl apply -f deploy/kind/relations/spicedb-kind-setup/relations-api/svc.yaml



echo "Waiting for all pods to be fully ready..."
while true; do
  PODS_READY=$(kubectl get pods --no-headers | awk '{print $2}' | grep -v '^1/1$' | wc -l)
  if [ "$PODS_READY" -eq 0 ]; then
    echo "All pods are ready!"
    break
  else
    echo "Waiting for pods to be ready... ($PODS_READY not ready yet)"
    kubectl get pods
    sleep 30
  fi
done

kubectl apply -f deploy/kind/e2e/e2e-batch.yaml
echo "Setup complete."
sleep 20
