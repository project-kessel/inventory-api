# RDS Disk Usage Alerts Related to WAL Growth

**Related Alerts**:
* TransactionLogDiskSpaceUsageHigh
* ReplicationSlotLagOrUsageAnomaly

>[!NOTE]
> There is a known issue that whenever the Kafka Connect that facilitates Debezium is restarted, it triggers this WAL growth issue to occur when there is no active API traffic after the restart. This issue will go away as the service is utilized more, but in the interim, a CronJob has been deployed to update a resource every hour and prevent this issue. The CronJob is deployed with the [service](https://github.com/project-kessel/inventory-api/blob/441a0a0b7210b5c837bc4aa40414ea80f2d0ecc3/deploy/kessel-inventory.yaml#L102) and utilizes a [script](https://github.com/project-kessel/inventory-api/blob/main/scripts/heartbeat-fix.sh) in this repo. More details can be found in [RHCLOUD-40690](https://issues.redhat.com/browse/RHCLOUD-40690)

### Reason

`TransactionLogDiskSpaceUsageHigh` and `ReplicationSlotLagOrUsageAnomaly` alerts are related to the use of logical replication slots for Debezium and would only fire if disk usage by WAL increased substantially over a period of time. Increases in disk usage by the WAL log are generally related to either a replication slot being lost/inactive or the replication slot being idle

The Debezium connector is configured with a heartbeat query that exists to prevent these issues from occurring but there are scenarios where it has happened and monitoring is required to avoid full disk usage. More specifically, we've seen this issue occur whenever the Kafka Connect pod leveraged by Debezium is restarted/redeployed and there is no traffic on the API. The WAL growth is generally due to AWS heartbeat messages consuming disk space due to an increase in number of WAL files.

`ReplicationSlotLagOrUsageAnomaly` checks for growth in replication slot disk usage over the course of an hour. RDS periodically writes heartbeats into the rdsadmin database every 5 minutes. Each heartbeat equates to a 64MB WAL file. Over the course of an hour that is 0.75GB, this alert fires when the change in disk usage over an hour is greater than 0.7GB to detect the issue.

`TransactionLogDiskSpaceUsageHigh` is a backup alert to ensure if the previous alert does not catch the issue or the rate of growth changes due to AWS default changes, it alerts when WAL disk usage exceeds 54GB.

### Verify and Remediate

As mentioned, the configured heartbeat messages in Debezium prevent this issue occurring most of the time. The only current known issue where this fails is when the Platform-MQ Kafka Connect pod is restarted for any reason (deployment update, delete/recreate, etc). Generally within an hour of a Connect pod restart, there is enough WAL growth to trigger the `ReplicationSlotLagOrUsageAnomaly` alert.

When the alert fires, verify if the Connect pod has restarted recently

```shell
oc get pod -n platform-mq-stage platform-kafka-connect-connect-0
```

If the **AGE** column indicates the pod is new, this is likely the culprit. If not, investigate the Inventory API pod logs and review other consumer related runbooks in this repo.

To remediate the WAL growth issue due to a Connect pod restart, the fix is to trigger some traffic through debezium by making some API calls which triggers outbox writes and consumption.

**Prerequisites**
1. Access to Stage cluster and `kessel-stage` namespace
2. Inventory service account client credentials (in vault)
3. The `load-generator` script from [Inventory API repo](https://github.com/project-kessel/inventory-api/blob/main/scripts/load-generator.sh)


**Process**:
1. Grab the service account client id and secret from vault

Vault Secret Path: `insights/secrets/insights-stage/kessel/inventory-sa`

2. Fetch a bearer token using the client id and secret

```shell
export TOKEN=$(curl 'https://sso.stage.redhat.com/auth/realms/redhat-external/protocol/openid-connect/token' \
    -H 'Content-Type: application/x-www-form-urlencoded' \
    --data-urlencode 'grant_type=client_credentials' \
    --data-urlencode 'client_id=CLIENT_ID_HERE' \
    --data-urlencode 'client_secret=CLIENT_SECRET_HERE' | jq -r '.access_token')
```

3. Login to the cluster and port-forward the inventory-api service

```shell
oc -n kessel-stage port-forward svc/kessel-inventory-api 8000:8000
```

4. Create and Delete a resource using the API

>[!NOTE]
> The load-generator script is not a hard requirement, but it's a useful tool if you are not familiar with the API and just need to quickly create/update/delete a resource. More explicit details of using the API are in our [README](../../../README.md)

```shell
./load-generator.sh -n 1 -i 1 -p 8000
```

The script will create exactly one resource, update it, then delete it, ensuring nothing is leftover. This is enough to generate outbox traffic. The disk usage should drop shortly after and should be visible in the RDS Dashboard linked to the alert. The alert generally drops a few mins later. If more traffic is needed, just re-run the script.
