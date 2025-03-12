# Testing Debezium in Ephemeral

## Prerequistes
You'll need:
1) [Bonfire](https://github.com/RedHatInsights/bonfire?tab=readme-ov-file#installing-locally) cli
2) [`psql`](https://www.postgresql.org/download/) cli
3) [`jq`](https://github.com/jqlang/jq?tab=readme-ov-file#installation) cli
4) Access to the Ephemeral cluster (which will require Red Hat Corp VPN)

## Deploy Inventory

Using Bonfire to deploy inventory will provide us with all the systems we need to test debezium. This includes the Postgres database that Debezium will capture changes from.

To Deploy:

`bonfire deploy kessel -C kessel-inventory --no-get-dependencies`

This will deploy Inventory alone with no Relations API. If you need Relations (future state testing), remove the `--no-get-dependencies` flag and it will also get deployed

> NOTE: By default ephemeral env's are nuked after 1 hour.
> If you need more time, extend that duration now before you forget: `bonfire namespace extend -d 4h # sets it to 4 hours`

## Prep for Debezium Deployment

The Ephemeral environment already provides a Kafka Cluster and Kafka Connect cluster with Debezium installed, so all that is needed is the Kakfa Connector to configure Debezium for our database and topics

### Database Setup

To deploy the connector, we need the database credentials for the Postgres database created. You can easily export those values by sourcing the `debezium-db-config-env` file: `source deploy/debezium/debezium-db-config-env`

Until Inventory API handles configuring the outbox table on used by Debezium, this step must be manually done ahead of time.

```shell
# Port forward postgres to your laptop
oc port-forward svc/kessel-inventory-db 5432:5432

# In another terminal tab/window, setup the outbox table
# Make sure you have the creds exported first
source deploy/debezium/debezium-db-config-env

psql "postgresql://${DB_USER}:${DB_PASSWORD}@localhost:${DB_PORT}/${DB_NAME}" -f deploy/debezium/outbox.sql

# You can validate the table is properly configured with:
psql "postgresql://${DB_USER}:${DB_PASSWORD}@localhost:${DB_PORT}/${DB_NAME}" -c "\d+ outbox_events"
```

## Deploy the Debezium Connector

To deploy Debezium, process and apply the OpenShift template, passing the environment variables sourced earlier

```shell
oc process --local -f deploy/debezium/debezium-connector.yaml \
    -p DB_NAME=$DB_NAME \
    -p DB_HOSTNAME=$DB_HOSTNAME \
    -p DB_PORT=$DB_PORT \
    -p DB_USER=$DB_USER \
    -p DB_PASSWORD=$DB_PASSWORD \
    -p KAFKA_CONNECT_INSTANCE=$KAFKA_CONNECT_INSTANCE | oc apply -f -
```

This should deploy the Kafka Connector which can be checked using:

```shell
$ oc get kctr kessel-inventory-source-connector

# example output -- Ready status should be 'True' if there are no errors
NAME                                CLUSTER                         CONNECTOR CLASS                                      MAX TASKS   READY
kessel-inventory-source-connector   env-ephemeral-uupuy9-dc11006e   io.debezium.connector.postgresql.PostgresConnector   1           True
```

If the Connector is Ready, everything is all setup for testing. If the connector is not ready, review the connector object and see if there are any errors or issues

```shell
oc describe kctr kessel-inventory-source-connector
```

## Testing the Debezium Connector

To test the Debezium Connector, we need to create a record in the outbox table with the correct `aggregatetype` and `payload` for the usecase:
* To produce resource creation/change events, the `aggregatetype` should be `kessel.resources` and the payload should contain an event using our [current event format](https://github.com/project-kessel/inventory-api/blob/4e924e0a731501c51dc523821f66070e3595d4f0/internal/eventing/api/event.go#L13)
* To produce tuple creation/change events, the `aggregatetype` should be `kessel.tuples`, and the payload should be a JSON request body for creating a tuple

### Setup

```shell
# Port forward postgres to your laptop
oc port-forward svc/kessel-inventory-db 5432:5432

# In another terminal tab/window, export DB creds if not already
source deploy/debezium/debezium-db-config-env
```

### Create Records in the Outbox table

```shell
# Create the tuple record in the outbox table
psql "postgresql://${DB_USER}:${DB_PASSWORD}@localhost:${DB_PORT}/${DB_NAME}" -f deploy/debezium/sample-tuple.sql

# Create a resource record in the outbox table
psql "postgresql://${DB_USER}:${DB_PASSWORD}@localhost:${DB_PORT}/${DB_NAME}" -f deploy/debezium/sample-resource.sql
```

### Check that the Messages were Produced

```shell
# Capture the bootstrap server address -- you will need it for next steps
oc get svc -o json | jq -r '.items[] | select(.metadata.name | test("^env-ephemeral.*-kafka-bootstrap")) | "\(.metadata.name).\(.metadata.namespace).svc"'

# rsh into the Connect pod
oc rsh ${KAFKA_CONNECT_INSTANCE}-connect-0

# Use the consumer script to look at messages for each topic
bin/kafka-console-consumer.sh --bootstrap-server <YOUR_BOOTSTRAP_SERVER>:9092 --topic outbox.event.kessel.tuples --from-beginning

# note, the consumer process runs continously, to exit, hit Ctrl+c

bin/kafka-console-consumer.sh --bootstrap-server <YOUR_BOOTSTRAP_SERVER>:9092 --topic outbox.event.kessel.resources --from-beginning
```

## Cleanup

To tear it all down: `bonfire namespace release`
