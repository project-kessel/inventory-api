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

#kubectl create configmap inventory-api-psks --from-file=config/psks.yaml
#
#kubectl apply -f https://strimzi.io/install/latest\?namespace\=default
#kubectl apply -f deploy/kind/inventory/kessel-inventory.yaml
#kubectl apply -f deploy/kind/inventory/invdatabase.yaml
#kubectl apply -f deploy/kind/e2e/e2e-batch.yaml
#kubectl apply -f deploy/kind/inventory/strimzi.yaml

# Create the kessel namespace
kubectl create namespace kessel

# Deploy ConfigMap for inventory-api
kubectl create configmap inventory-api-psks --from-file=config/psks.yaml

# Deploy Inventory dependencies
kubectl apply -f https://strimzi.io/install/latest\?namespace\=default
kubectl apply -f deploy/kind/inventory/kessel-inventory.yaml
kubectl apply -f deploy/kind/inventory/invdatabase.yaml
kubectl apply -f deploy/kind/inventory/strimzi.yaml

kubectl apply -f deploy/kind/relations/spicedb-kind-setup/bundle.yaml

kubectl apply -f https://projectcontour.io/quickstart/contour.yaml
kubectl get crd httpproxies.projectcontour.io

# Deploy SpiceDB and Relations-API in the kessel namespace
kubectl apply -f deploy/kind/relations/spicedb-kind-setup/postgres/secret.yaml
kubectl apply -f deploy/kind/relations/spicedb-kind-setup/postgres/postgresql.yaml
kubectl apply -f deploy/kind/relations/spicedb-kind-setup/postgres/storage.yaml
kubectl apply -f deploy/kind/relations/spicedb-kind-setup/spicedb-cr.yaml
kubectl apply -f deploy/kind/relations/spicedb-kind-setup/svc-ingress.yaml
kubectl apply -f deploy/kind/relations/spicedb-kind-setup/relations-api/secret.yaml
kubectl apply -f deploy/kind/relations/spicedb-kind-setup/relations-api/deployment.yaml
kubectl apply -f deploy/kind/relations/spicedb-kind-setup/relations-api/svc.yaml

echo "Setup complete. Inventory API, Relations-API, and SpiceDB are running!"