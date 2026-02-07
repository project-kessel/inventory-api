#!/bin/bash

# prints a handy help menu with usage
help_me() {
    echo "USAGE: load-generator.sh {-n <NUM_RUNS>} {-i <INTERVAL>} {-p <PORT_NUM>} [-h]"
    echo "load-generator: creates load on inventory API by creating, updating, and deleting resources for test purposes"
    echo "It requires a local running Inventory API or port-forwarding connection to one in ephemeral (using port defined with -p)"
    echo ""
    echo "REQUIRED ARGUMENTS:"
    echo "  -n NUM_RUNS: The number of times to run a test loop (one run is one create, update, and delete loop"
    echo "  -i INTERVAL: Amount of time between runs"
    echo "  -p PORT_NUM: Port number used for Inventory API"
    echo ""
    echo "OPTIONS:"
    echo "  -h Prints usage information"
    echo "  -c: If set, generator performs create operations only, no updates or delete"
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

while getopts "n:i:p:hc" flag; do
    case "${flag}" in
        n) NUM_RUNS=${OPTARG};;
        i) INTERVAL=${OPTARG};;
        p) PORT_NUM=${OPTARG};;
        c) CREATE_ONLY=true;;
        h) help_me;;
    esac
done

if [[  -z "${NUM_RUNS}" || -z "${INTERVAL}"  || -z "${PORT_NUM}" ]]; then
  echo "Error: required arguments not provided"
  help_me
fi

LIVEZ_URL="localhost:${PORT_NUM}/api/kessel/v1/livez"
INVENTORY_URL="localhost:${PORT_NUM}/api/kessel/v1beta2/resources"

for ((i = 0 ; i < ${NUM_RUNS} ; i++)); do
  livez_check
  REPORTER_INSTANCE_ID=$(uuidgen)
  WORKSPACE_ID=$(uuidgen)
  LOCAL_RESOURCE_ID=$(uuidgen)
  SATELLITE_ID=$(uuidgen)
  SUB_MANAGER_ID=$(uuidgen)
  INSIGHTS_ID=$(uuidgen)
  ANSIBLE_HOST="host-${i}"

  REQUEST=$(jq -c --null-input \
    --arg reporter_instance_id "$REPORTER_INSTANCE_ID" \
    --arg workspace_id  "$WORKSPACE_ID" \
    --arg local_resource_id "$LOCAL_RESOURCE_ID" \
    --arg satellite_id "$SATELLITE_ID" \
    --arg sub_manager_id "$SUB_MANAGER_ID" \
    --arg insights_id "$INSIGHTS_ID" \
    --arg ansible_host "$ANSIBLE_HOST" \
    '{"type":"host","reporterType":"hbi","reporterInstanceId":$reporter_instance_id,"representations":{"metadata":{"localResourceId":$local_resource_id,"apiHref":"https://apiHref.com/","consoleHref":"https://www.console.com/","reporterVersion":"2.7.16"},"common":{"workspace_id":$workspace_id},"reporter":{"satellite_id":$satellite_id,"subscription_manager_id":$sub_manager_id,"insights_id":$insights_id,"ansible_host":$ansible_host}}}')

  DELETE_REQUEST=$(jq -c --null-input \
    --arg local_resource_id "$LOCAL_RESOURCE_ID" \
    '{"reference":{"resource_type":"host","resource_id":$local_resource_id,"reporter":{"type":"hbi"}}}')

  echo "Creating resource..."
  curl -H "Content-Type: application/json" -H "Authorization: Bearer $TOKEN" -d $REQUEST $INVENTORY_URL

  if [[ ! "$CREATE_ONLY" == "true" ]]; then
    echo "Updating resource..."
    curl -H "Content-Type: application/json" -H "Authorization: Bearer $TOKEN" -d $REQUEST $INVENTORY_URL

    echo "Deleting resource..."
    curl -X DELETE -H "Content-Type: application/json" -H "Authorization: Bearer $TOKEN" -d $DELETE_REQUEST $INVENTORY_URL
  fi

  sleep $INTERVAL
done
