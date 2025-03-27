#!/bin/bash

# prints a handy help menu with usage
help_me() {
    echo "USAGE: load-generator.sh {-n <NUM_RUNS>} {-i <INTERVAL>} {-p <PORT_NUM>} [-h]"
    echo "load-generator: creates load on inventory API by creating, updating, and deleting resources for test purposes"
    echo "It requires a local running Inventory API or port-forwarding connection to one in ephemeral (usig port defined with -p)"
    echo ""
    echo "REQUIRED ARGUMENTS:"
    echo "  -n NUM_RUNS: The number of times to run a test loop (one run is one create, update, and delete loop"
    echo "  -i INTERVAL: Amount of time between runs"
    echo "  -p PORT_NUM: Port number used for Inventory API"
    echo ""
    echo "OPTIONS:"
    echo "  -h Prints usage information"
    echo ""
    echo "EXAMPLE:"
    echo "# Run 5 test loops with a 3 second break between tests"
    echo "  load-generator.sh -n 5 -i 3"
    exit 0
}

livez_check() {
  STATUS=$(curl -s $LIVEZ_URL | jq -r '.status')
  if [[ "${STATUS}" != "OK" ]]; then
    echo "LiveZ check failed -- is Inventory running or port-forwarded?"
    exit 1
  fi
}

while getopts "n:i:p:h" flag; do
    case "${flag}" in
        n) NUM_RUNS=${OPTARG};;
        i) INTERVAL=${OPTARG};;
        p) PORT_NUM=${OPTARG};;
        h) help_me;;
    esac
done

if [[  -z "${NUM_RUNS}" || -z "${INTERVAL}"  || -z "${PORT_NUM}" ]]; then
  echo "Error: required arguments not provided"
  help_me
fi

LIVEZ_URL="localhost:${PORT_NUM}/api/inventory/v1/livez"
INVENTORY_URL="localhost:${PORT_NUM}/api/inventory/v1beta1/resources/notifications-integrations"

for ((i = 0 ; i < ${NUM_RUNS} ; i++)); do
  livez_check
  WORKSPACE_ID=$(uuidgen)
  LOCAL_RESOURCE_ID=$(uuidgen)

  REQUEST=$(jq -c --null-input \
    --arg workspace_id  "$WORKSPACE_ID" \
    --arg local_resource_id "$LOCAL_RESOURCE_ID" \
    '{"integration":{"metadata":{"workspace_id":$workspace_id,"resource_type":"notifications/integration"},"reporter_data":{"reporter_instance_id":"service-account-1","reporter_type":"NOTIFICATIONS","local_resource_id":$local_resource_id}}}')

  DELETE_REQUEST=$(jq -c --null-input \
    --arg local_resource_id "$LOCAL_RESOURCE_ID" \
    '{"reporter_data":{"reporter_type":"NOTIFICATIONS","local_resource_id":$local_resource_id}}')

  echo "Creating resource..."
  curl -H "Content-Type: application/json" -d $REQUEST $INVENTORY_URL
  sleep 1

  echo "Updating resource..."
  curl -X PUT -H "Content-Type: application/json" -d $REQUEST $INVENTORY_URL
  sleep 1
  echo "Deleting resource..."
  curl -X DELETE -H "Content-Type: application/json" -d $DELETE_REQUEST $INVENTORY_URL
  sleep 1

  sleep $INTERVAL
done
