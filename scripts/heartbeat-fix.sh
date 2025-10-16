#!/bin/bash

# RHCLOUD-40690 temp heartbeat fix to prevent WAL growth due to ongoing debezium issue

SSO_RESP=$(curl "${SSO_URL}" -H 'Content-Type: application/x-www-form-urlencoded' --data-urlencode 'grant_type=client_credentials' --data-urlencode "client_id=${SA_CLIENT_ID}" --data-urlencode "client_secret=${SA_CLIENT_SECRET}")
TOKEN=$(echo $SSO_RESP | awk -F ':"' '{print $2}' | cut -d '"' -f 1);
INVENTORY_URL="https://kessel-inventory-api:8000/api/inventory/v1beta2/resources";
BODY='{"type":"host","reporterType":"HBI","reporterInstanceId":"3088be62-1c60-4884-b133-9200542d0b3f","representations":{"metadata":{"localResourceId":"dbz-issue-workaround-RHCLOUD-40690","apiHref":"https://apiHref.com/","consoleHref":"https://www.console.com/","reporterVersion":"2.7.16"},"common":{"workspace_id":"dbz-issue-workaround-RHCLOUD-40690"},"reporter":{"satellite_id":"2c4196f1-0371-4f4c-8913-e113cfaa6e67","sub_manager_id":"af94f92b-0b65-4cac-b449-6b77e665a08f","insights_id":"05707922-7b0a-4fe6-982d-6adbc7695b8f","ansible_host":"host-1"}}}';
curl -X POST -H "Content-Type: application/json" -H "Authorization: bearer $TOKEN" --cacert /serving-certs/tls.crt -d $BODY $INVENTORY_URL
