#!/bin/bash

BASE_CONFIG_PATH=development/configs/monitoring/base-ephemeral-prometheus.yml
CONFIG_PATH=development/configs/monitoring/ephemeral-prometheus.yml

NAMESPACE=$(oc project -q 2> /dev/null)
IS_ACTIVE=$(oc get pods 2> /dev/null --no-headers | wc -l)
if [[ ! "${NAMESPACE}" == "ephemeral"* ]] || [[ "$IS_ACTIVE" -eq 0 ]]; then
    echo "Current target namespace is not an ephemeral namespace or has no deployments"
    echo "Are you working in Ephemeral? Do you have a currently deployed Kessel yet"
    echo "Current project: $NAMESPACE"
    exit 1
fi

INVENTORY_PODS=$(oc get deploy kessel-inventory-api -o jsonpath='{.status.readyReplicas}')
KIC_PODS=$(oc get deploy kessel-inventory-consumer-service -o jsonpath='{.status.readyReplicas}')
KKC_PODS=$(oc get sps kessel-kafka-connect-connect -o jsonpath='{.status.readyPods}')

# clean up all files/volumes and setup config
echo "Removing old grafana volume if it exists..."
podman volume rm development_grafana-storage
rm -f $CONFIG_PATH
cp $BASE_CONFIG_PATH $CONFIG_PATH

# Inventory is always expected to be deployed but check just in case
if [[ "$INVENTORY_PODS" -ge 1 ]]; then
    echo "Configuring metrics route for Inventory API..."
    oc expose svc/kessel-inventory-api --target-port public --path "/metrics" --name kessel-inventory-metrics
    EPHEM_INVENTORY_ROUTE=$(oc get route kessel-inventory-metrics -o jsonpath='{.spec.host}')
    echo "Updating prometheus config..."
    sed -i'' "s/EPHEM_INVENTORY_ROUTE/${EPHEM_INVENTORY_ROUTE}/" $CONFIG_PATH
fi

# KIC is only deployed if explicitly done
if [[ "$KIC_PODS" -ge 1 ]]; then
    echo "Configuring metrics route for KIC..."
    oc expose svc/kessel-inventory-consumer-service --target-port metrics --path "/metrics" --name kessel-inventory-consumer-metrics
    EPHEM_KIC_ROUTE=$(oc get route kessel-inventory-consumer-metrics -o jsonpath='{.spec.host}')
    echo "Updating prometheus config..."
    sed -i'' "s/EPHEM_KIC_ROUTE/${EPHEM_KIC_ROUTE}/" $CONFIG_PATH
else
    # Remove the entire job block to avoid invalid scrape targets if KIC is not also deployed
    sed -i'' '/job_name: kessel-inventory-consumer/,/^[[:space:]]*-[[:space:]]job_name\|^[^[:space:]]/{ /job_name: kessel-inventory-consumer/!{ /^[[:space:]]*-[[:space:]]job_name\|^[^[:space:]]/!d } }' $CONFIG_PATH
fi

# KKC is always expected to be deployed but check just in case
if [[ "$KKC_PODS" -ge 1 ]]; then
    echo "Configuring metrics service and route for KKC"
    oc expose pod/kessel-kafka-connect-connect-0 --port 9404 --target-port 9404 --name kkc-metrics
    oc expose svc/kkc-metrics --target-port 9404 --path "/metrics" --name kkc-metrics
    EPHEM_KKC_ROUTE=$(oc get route kkc-metrics -o jsonpath='{.spec.host}')
    echo "Updating prometheus config..."
    sed -i'' "s/EPHEM_KKC_ROUTE/${EPHEM_KKC_ROUTE}/" $CONFIG_PATH
fi
