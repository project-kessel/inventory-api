#!/bin/bash

# This script has been copied from:
# https://github.com/stolostron/multicluster-global-hub/blob/6f83a5f957577edde9edb30a7690feb209817b77/test/kessel_e2e/setup/e2e_setup.sh
# The function `deployGlobalHub` to use the upstream version of MCGH

set -exo pipefail

source ./scripts/check_docker_podman.sh

MCGH_REPO="multicluster-global-hub"

function initKinDCluster() {
  clusterName="$1"
  if [[ $(kind get clusters | grep "^${clusterName}$" || true) != "${clusterName}" ]]; then
    kind create cluster --name "$clusterName" --wait 1m
    currentDir="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
    kubectl config view --context="kind-${clusterName}" --minify --flatten > ${currentDir}/kubeconfig-${clusterName}
  fi
}

function pushInventoryImageKind() {
  clusterName="$1"
  rm -f inventory-api.tar
  ${DOCKER} build --arch amd64 . -t localhost/inventory-api:latest
  ${DOCKER} save -o inventory-api.tar localhost/inventory-api:latest
  kind load image-archive inventory-api.tar --name "$clusterName"
}

function pushtestInventoryImageKind() {
  clusterName="$1"
  ${DOCKER} build --arch amd64 -t localhost/inventory-e2e-tests:latest -f Dockerfile-e2e
  ${DOCKER} save -o inventory-e2e-tests.tar localhost/inventory-e2e-tests:latest
  kind load image-archive inventory-e2e-tests.tar --name "$clusterName"
}

enableRouter() {
  kubectl create ns openshift-ingress --dry-run=client -o yaml | kubectl --context "$1" apply -f -
  GIT_PATH="https://raw.githubusercontent.com/openshift/router/release-4.16"
  kubectl --context "$1" apply -f $GIT_PATH/deploy/route_crd.yaml
  # pacman application depends on route crd, but we do not need to have route pod running in the cluster
  # kubectl apply -f $GIT_PATH/deploy/router.yaml
  # kubectl apply -f $GIT_PATH/deploy/router_rbac.yaml
}

enableServiceCA() {
  HUB_OF_HUB_NAME=$2
  # apply service-ca
  kubectl --context $1 label node ${HUB_OF_HUB_NAME}-control-plane node-role.kubernetes.io/master=
  kubectl --context $1 apply -f ${MCGH_REPO}/test/kessel_e2e/setup/service-ca-crds/
  kubectl --context $1 create ns openshift-config-managed
  kubectl --context $1 apply -f ${MCGH_REPO}/test/kessel_e2e/setup/service-ca/
}

# deploy olm
function enableOLM() {
  NS=olm
  csvPhase=$(kubectl --context "$1" get csv -n "${NS}" packageserver -o jsonpath='{.status.phase}' 2>/dev/null || echo "Waiting for CSV to appear")
  if [[ "$csvPhase" == "Succeeded" ]]; then
    echo "OLM is already installed in ${NS} namespace. Exiting..."
    exit 1
  fi

  GIT_PATH="https://raw.githubusercontent.com/operator-framework/operator-lifecycle-manager/v0.28.0"
  kubectl --context "$1" apply -f "${GIT_PATH}/deploy/upstream/quickstart/crds.yaml"
  kubectl --context "$1" wait --for=condition=Established -f "${GIT_PATH}/deploy/upstream/quickstart/crds.yaml" --timeout=60s
  kubectl --context "$1" apply -f "${GIT_PATH}/deploy/upstream/quickstart/olm.yaml"

  # apply proxies.config.openshift.io which is required by olm
  kubectl --context "$1" apply -f "https://raw.githubusercontent.com/openshift/api/master/payload-manifests/crds/0000_03_config-operator_01_proxies.crd.yaml"

  retries=60
  csvPhase=$(kubectl --context "$1" get csv -n "${NS}" packageserver -o jsonpath='{.status.phase}' 2>/dev/null || echo "Waiting for CSV to appear")
  while [[ $retries -gt 0 && "$csvPhase" != "Succeeded" ]]; do
    echo "csvPhase: ${csvPhase}"
    sleep 2
    retries=$((retries - 1))
    csvPhase=$(kubectl --context "$1" get csv -n "${NS}" packageserver -o jsonpath='{.status.phase}' 2>/dev/null || echo "Waiting for CSV to appear")
  done
  kubectl --context "$1" rollout status -w deployment/packageserver --namespace="${NS}" --timeout=60s

  if [ $retries == 0 ]; then
    echo "CSV \"packageserver\" failed to reach phase succeeded"
    exit 1
  fi
  echo "CSV \"packageserver\" install succeeded"
}

# deploy global hub
function deployGlobalHub() {
    # patch inventory api image
    echo "- path: manager_inventory_api_image_patch.yaml" >> ${MCGH_REPO}/operator/config/manager/kustomization.yaml
    cp scripts/assets/manager_inventory_api_image_patch.yaml ${MCGH_REPO}/operator/config/manager/manager_inventory_api_image_patch.yaml

    (
        cd "${MCGH_REPO}/operator"
        make deploy
    )

    # deploy serviceMonitor CRD
    kubectl --context "$1" apply -f "${MCGH_REPO}/test/manifest/crd/0000_04_monitoring.coreos.com_servicemonitors.crd.yaml"

    # get the kind cluster ip address
    global_hub_node_ip=$(kubectl  --context "$1" get node -o jsonpath='{.items[0].status.addresses[?(@.type=="InternalIP")].address}')

    cat <<EOF | kubectl --context "$1" apply -f -
apiVersion: operator.open-cluster-management.io/v1alpha4
kind: MulticlusterGlobalHub
metadata:
  annotations:
    global-hub.open-cluster-management.io/catalog-source-name: operatorhubio-catalog
    global-hub.open-cluster-management.io/catalog-source-namespace: olm
    global-hub.open-cluster-management.io/with-inventory: ""
    global-hub.open-cluster-management.io/enable-kraft: ""
    global-hub.open-cluster-management.io/kafka-broker-advertised-host: "$global_hub_node_ip"
  name: multiclusterglobalhub
  namespace: multicluster-global-hub
spec:
  availabilityConfig: High
  dataLayer:
    kafka:
      topics:
        specTopic: gh-spec
        statusTopic: gh-event.*
      storageSize: 1Gi
    postgres:
      retention: 18m
      storageSize: 1Gi
  enableMetrics: false
  imagePullPolicy: IfNotPresent
EOF
}

function wait_cmd() {
    cmd="$1"
    echo "Waiting for: $cmd"

    retries=100
    while [[ $retries -gt 0 ]]; do
        eval "$cmd" &> /dev/null && echo && break
        echo -n "."
        sleep 10
        retries=$((retries - 1))
    done
    if [[ $retries == 0 ]]; then
        echo "Command \"$cmd\" failed to return successfully"
        exit 1
    fi
}

function wait_global_hub_ready() {
  wait_cmd "kubectl get deploy/multicluster-global-hub-manager -n multicluster-global-hub --context $1"
  kubectl wait deploy/multicluster-global-hub-manager -n multicluster-global-hub --for condition=Available=True --timeout=600s --context "$1"
}

# Clone repo if not already cloned
[[ ! -d "${MCGH_REPO}" ]] && git clone https://github.com/stolostron/multicluster-global-hub.git "${MCGH_REPO}"

initKinDCluster global-hub
pushInventoryImageKind global-hub
pushtestInventoryImageKind global-hub
enableRouter kind-global-hub
enableServiceCA kind-global-hub global-hub
enableOLM kind-global-hub
deployGlobalHub kind-global-hub global-hub
wait_global_hub_ready kind-global-hub
