#!/bin/bash

# RHCLOUD-40690 temp heartbeat fix to prevent WAL growth due to ongoing debezium issue

SSO_RESP=$(curl "${SSO_URL}" -H 'Content-Type: application/x-www-form-urlencoded' --data-urlencode 'grant_type=client_credentials' --data-urlencode "client_id=${SA_CLIENT_ID}" --data-urlencode "client_secret=${SA_CLIENT_SECRET}")
TOKEN=$(echo $SSO_RESP | awk -F ':"' '{print $2}' | cut -d '"' -f 1);
INVENTORY_URL="kessel-inventory-api:8000/api/inventory/v1beta1/resources/notifications-integrations";
BODY='{"integration":{"metadata":{"workspace_id":"dbz-issue-workaround-RHCLOUD-40690","resource_type":"notifications/integration"},"reporter_data":{"reporter_instance_id":"service-account-1","reporter_type":"NOTIFICATIONS","local_resource_id":"dbz-issue-workaround-RHCLOUD-40690"}}}';
curl -X PUT -H "Content-Type: application/json" -H "Authorization: bearer $TOKEN" -d $BODY $INVENTORY_URL
