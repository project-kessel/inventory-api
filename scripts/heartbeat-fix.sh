#!/bin/bash

# RHCLOUD-40690 temp heartbeat fix to prevent WAL growth due to ongoing debezium issue

# Commenting out auth bits while auth is disabled
#SSO_RESP=$(curl "${SSO_URL}" -H 'Content-Type: application/x-www-form-urlencoded' --data-urlencode 'grant_type=client_credentials' --data-urlencode "client_id=${SA_CLIENT_ID}" --data-urlencode "client_secret=${SA_CLIENT_SECRET}")
#TOKEN=$(echo $SSO_RESP | awk -F ':"' '{print $2}' | cut -d '"' -f 1);
INVENTORY_URL="kessel-inventory-api:9000"
ENDPOINT="kessel.inventory.v1beta2.KesselInventoryService.ReportResource"
BODY='{"type":"host","reporter_type":"hbi","reporter_instance_id":"3088be62-1c60-4884-b133-9200542d0b3f","representations":{"metadata":{"local_resource_id":"dbz-issue-workaround-RHCLOUD-40690","api_href":"https://apiHref.com/","console_href":"https://www.console.com/","reporter_version":"2.7.16"},"common":{"workspace_id":"dbz-issue-workaround-RHCLOUD-40690"},"reporter":{"satellite_id":"2c4196f1-0371-4f4c-8913-e113cfaa6e67","sub_manager_id":"af94f92b-0b65-4cac-b449-6b77e665a08f","insights_id":"05707922-7b0a-4fe6-982d-6adbc7695b8f","ansible_host":"host-1"}}}';
grpcurl -plaintext -d "$BODY" "$INVENTORY_URL" "$ENDPOINT"
