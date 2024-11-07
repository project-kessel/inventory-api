#!/bin/bash
set -e

source ./scripts/check_docker_podman.sh

kind create cluster --name inventory-cluster

kubectl create secret docker-registry redhat-registry-secret \
  --docker-server=$DOCKER_SERVER \
  --docker-username=$QUAY_USERNAME \
  --docker-password=$QUAY_PASSWORD \
  --docker-email=$QUAY_EMAIL

kubectl create serviceaccount default
kubectl patch serviceaccount default -p '{"imagePullSecrets": [{"name": "redhat-registry-secret"}]}'

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
kubectl apply -f deploy/kind/e2e/e2e-batch.yaml
kubectl apply -f deploy/kind/inventory/strimzi.yaml
