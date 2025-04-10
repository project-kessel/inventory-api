# Testing Inventory in in Ephemeral with Debezium

> Note: This process is currently very specific to feature branch work being done on [feature-RHCLOUD-38543](https://github.com/project-kessel/inventory-api/tree/feature-RHCLOUD-38543) and is not applicable to the main branch

## Prerequistes
You'll need:
1) [Bonfire](https://github.com/RedHatInsights/bonfire?tab=readme-ov-file#installing-locally) cli
2) [`jq`](https://github.com/jqlang/jq?tab=readme-ov-file#installation) cli
3) [`grpcurl`](https://github.com/fullstorydev/grpcurl?tab=readme-ov-file#installation) cli
4) A personal build of the Inventory container pushed to your Quay
5) Local bonfire config to set the correct image and deployment file
6) Access to the Ephemeral cluster (which will require Red Hat Corp VPN)


### Inventory Image

The changes in this feature branch are not automatically built by Konflux/AppSRE and require you to build and leverage your own image

To build your own image:
```shell
# For Linux
export IMAGE=your-quay-repo # if desired
make docker-build-push

# For MacOS
export QUAY_REPO_INVENTORY=your-quay-repo # required
podman login quay.io # required, this target assumes you are already logged in
make build-push-minimal
```

### Setup Local Bonfire Config

In order to deploy the correct image and manifests to Ephemeral, add or update any Kessel Inventory configuration in your local bonfire config ($HOME/.config/bonfire/config.yaml) to the below:

```yaml
apps:
- name: kessel
  components:
    - name: kessel-inventory
      host: local
      repo: </path/to/cloned/inventory-api-repo>
      path: deploy/kessel-inventory-ephem-w-debezium.yaml
      parameters:
        INVENTORY_IMAGE: quay.io/<YOUR_QUAY_REPO>/kessel-inventory
        IMAGE_TAG: "<YOUR_IMAGE_TAG>"
```

## Deploy Inventory

Using Bonfire to deploy inventory with the new image and deploy file will provide us with all the systems we need to test run Inventory with the new consumer process and Debezium connector.

To Deploy:

`bonfire deploy kessel -C kessel-inventory`

The full deployment will include:
* Inventory API
* Relations API
* Kafka Cluster
* Kafka Connect cluster with Debezium plugin
* Kafka Connector to configure Debezium
* Kafka Topics leveraged by the Consumer and Debezium
* Kafka User to use for authenticating with Kafka
* Postgres DB's for both services

> NOTE: By default ephemeral env's are nuked after 1 hour.
> If you need more time, extend that duration now before you forget: `bonfire namespace extend -d 4h # sets it to 4 hours`

Once the deployment completes, it may take a little bit of time for the Kafka setup to complete. You can validate all Kafka services are ready by checking them with:

 `make check-kafka-status`.

 When ready, all systems will output `True`

## Testing

When creating a resource (in this setup, a notifications integration) in Inventory API, the expected outcome should be:
* Resource is added to Inventory via API and reflected in Inventory DB
* Resource is created then removed from outbox tables
* Debezium captures the changes and produces a message to the resources and tuples outboxes
* The Inventory Consumer process captures the message in the tuples topic, and created the relationship in SpiceDB via Relations API
* The consumer captures the consistency token in the response and updates the resource in Inventory DB with the token


### Test Process

1) Create a Notification's Integration using the Inventory API

```shell
# port-forward the api
oc port-forward svc/kessel-inventory-api 8000:8000

# create the notification
make create-test-notification
```

2) Validate that Debezium captured the change and produced the expected messages

```shell
# Check Tuple messages -- it may take a few seconds before any messages appear
make check-tuple-messages

# !!! note, the consumer process runs continuously, to exit, hit Ctrl+c !!!

# Check Resource messages -- it may take a few seconds before any messages appear
make check-resource-messages

# !!! note, the consumer process runs continuously, to exit, hit Ctrl+c !!!
```

3) Validate that the Consumer processed the tuple message via Inventory pod logs

```shell
oc logs <INVENTORY_POD_NAME> | grep "consumed event"

# example expected output
INFO ts=2025-03-24T16:01:11Z caller=log/log.go:30 service.name=inventory-api service.version=0.1.0 trace.id= span.id= subsystem=inventoryConsumer msg=consumed event from topic outbox.event.kessel.tuples, partition 0 at offset 0: key = {"schema":{"type":"string","optional":false},"payload":"0195c8e2-edb8-7ead-a8a2-0ba7e275bc56"} value = {"schema":{"type":"string","optional":true,"name":"io.debezium.data.Json","version":1},"payload":"{\"subject\": {\"subject\": {\"id\": \"1234\", \"type\": {\"name\": \"workspace\", \"namespace\": \"rbac\"}}}, \"relation\": \"workspace\", \"resource\": {\"id\": \"4321\", \"type\": {\"name\": \"integration\", \"namespace\": \"notifications\"}}}"}
```

4) Validate the relation has been created in SpiceDB

```shell
# port-forward the relations api
oc port-forward svc/kessel-relations-api 9000:9000

# Read the tuple
make check-tuple
```

5) Validate the token has been updated in Inventory DB

```shell
# set env vars for DB creds
source scripts/debezium-config-env

# port forward DB
oc port-forward svc/kessel-inventory-db 5432:5432

# check the resources table
make check-token-update
```


The same process can also be applied to updating and deleting the notifications resource by changing the make command run in step 1:
* Update: `make update-test-notification`
* Delete: `make delete-test-notification`

## Cleanup

To tear it all down: `bonfire namespace release`

## Testing the Inventory Consumer with Authentication Enabled

The Kafka cluster deployed in this setup contains two listeners: One with no authentication and another that requires credentials using SASL_SCRAM. By default authentication is disabled for the consumer while the Connect cluster defaults to using the secure port. If you need to test the Inventory Consumer using authentication, the process slightly differs from the above.

1) Deploy using Bonfire with authentication disabled and using the insecure port for Kafka (default settings)

```yaml
consumer:
  enabled: true
  bootstrap-servers: inventory-kafka-kafka-bootstrap:9092 # port 9092 does not require auth
  topic: outbox.event.kessel.tuples
  auth:
    enabled: false
    # when enabled is false, the below settings are ignored and not needed, they are just provided to make it easier to enable
    security-protocol: sasl_plaintext
    sasl-mechanism: SCRAM-SHA-512
    sasl-username: inventory-consumer
    sasl-password: REPLACE_ME
```

2) Once the deployment completes, fetch the Kafka User password that we'll need to provide to the consumer

```shell
oc get secret inventory-consumer -o json | jq -r '.data.password | @base64d'
```

3) Update the consumer settings in `deploy/kessel-inventory-ephem-w-debezium.yaml` with the password and enable authentication

```yaml
consumer:
  enabled: true
  bootstrap-servers: inventory-kafka-kafka-bootstrap:9094 # port 9094 requires auth credentials
  topic: outbox.event.kessel.tuples
  auth:
    enabled: true # auth enabled
    security-protocol: sasl_plaintext
    sasl-mechanism: SCRAM-SHA-512
    sasl-username: inventory-consumer
    sasl-password: <PASSWORD-FROM-PREVIOUS-STEP>
```

4) Redeploy with bonfire: `bonfire deploy kessel -C kessel-inventory`

5) Kick the Inventory pod so it loads the new configuration: `oc delete pods -l pod=kessel-inventory-api`

The new running Inventory pod should now communicate with Kafka using the secure port and SCRAM credentials provided.
