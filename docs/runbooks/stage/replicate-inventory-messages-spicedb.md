# Manual Inventory DB replication to SpiceDB

## Prerequisites

Remediations covered in this guide will require the following:
1) [zed cli](https://github.com/authzed/zed?tab=readme-ov-file#getting-started)
2) [gabi cli](https://github.com/app-sre/gabi-cli) installed & gabi access on cluster
3) Some python3 version installed.
4) Access to the portforwarding into the running cluster. 

### Configuring Zed CLI

To install the `zed` CLI, refer to [Authzed's README](https://github.com/authzed/zed?tab=readme-ov-file#getting-started) for specific instructions based on your OS

In order to access SpiceDB using the `zed` cli, you'll need to configure a context for the SpiceDB endpoint. This will require a preshared token available in a Kubernetes secret on cluster.

To configure the context:

```shell
# capture the preshared key from spicedb-config secret
PSTK=$(oc get secret spicedb-config -o jsonpath='{.data.preshared_key}' | base64 -d)

# create the context with zed cli -- change ENV to the ENV you are targeting (stage, prod)
zed context set <ENV>-spicedb localhost:50051 $PSTK --insecure

# verify the context was created and is the current context
zed context list
```

To confirm the context is working:

```shell
# port-forward the spicedb service to your system
oc port-forward svc/kessel-relations-spicedb 50051:50051

# check the schema
zed schema read
```

The read command should dump out the entire SpiceDB schema. An error is provided if there are any connection issues or if no schema exists.

### Configuring gabi CLI

To install the `gabi` CLI, refer to [gabi's README](https://github.com/app-sre/gabi-cli) for specific instructions based on your OS

In order to access Inventory's DB using the `gabi` cli, you'll need to configure a profile for the stage gabi instance. Accessing the Gabi instance requires its route and a token used to log into the cluster.

If you don't already have a gabi config initialized, execute the `init` command: 
```shell
gabi config init
```
There are two settings we need to set up, the `URL` and `TOKEN`. You can grab the `URL` from
the `gabi-kessel` route and the `TOKEN` from the token used to log into the cluster.
Execute the following to set your url and token.
```shell
gabi config seturl <gabi-kessel-route>
gabi config settoken <sha256-login-token> 
```
You can check your current profile with `gabi config currentprofile`.
```shell
{
  "name": "default",
  "alias": "default",
  "url": "https://gabi-kessel.apps.cluster.example.com",
  "token": "sha256~xkXXX...",
  "current": true,
  "enable_history": true
}
```

To confirm `gabi` is working we can execute a basic query against the DB.
```shell
gabi exec "select id, created_at, reporter from resources limit 1"
[
  {
    "created_at": "2024-11-13T14:29:57.061157Z",
    "id": "X-X-X-X",
    "reporter": "{}"
  }
]
```

## Running the migration script.

We will need a dump of the offset messages that need to be reprocessed. To get a dump of processed messages,
we can spin up a kafka debug pod to read every message.

**Process**

```shell
# capture Kafka connection details for Kafka CLI commands later
BOOTSTRAP_SERVERS=$(oc get secret kessel-inventory -o json | jq -r '.data."cdappconfig.json"' | base64 -d | jq -r '.kafka.brokers[] | "\(.hostname):\(.port)"')
KAFKA_USER=$(oc get secret kessel-inventory -o json | jq -r '.data."cdappconfig.json"' | base64 -d | jq -r '.kafka.brokers[0].sasl.username')
KAFKA_PW=$(oc get secret kessel-inventory -o json | jq -r '.data."cdappconfig.json"' | base64 -d | jq -r '.kafka.brokers[0].sasl.password')

# run and exec into kafka pod
oc run kafka-debug --rm -i --tty --image quay.io/strimzi/kafka:0.45.0-kafka-3.9.0 --env BOOTSTRAP_SERVERS="$BOOTSTRAP_SERVERS" --env KAFKA_USER="$KAFKA_USER" --env KAFKA_PW="$KAFKA_PW" -- bash

# setup the config file with credentials information
cat <<EOF > /tmp/config.props
sasl.mechanism=SCRAM-SHA-512
security.protocol=SASL_SSL
sasl.jaas.config=org.apache.kafka.common.security.scram.ScramLoginModule required username="$KAFKA_USER" password="$KAFKA_PW";
EOF

# confirm current offset and lag
./bin/kafka-consumer-groups.sh --bootstrap-server $BOOTSTRAP_SERVERS --command-config /tmp/config.props --group inventory-consumer --describe

# print all processed messages
./bin/kafka-console-consumer.sh --topic outbox.event.kessel.tuples --from-beginning  --consumer.config /tmp/config.props --bootstrap-server $BOOTSTRAP_SERVERS --property print.offset=true --property print.key=true --property print.headers=true
```

Save these messages to some file like `kafkadump.txt`.

**WARNING:**
Before executing our script we must ensure the consumer is disabled such that no messages are processed at the same time
we are trying to manually replicate resources to SpiceDB. If you do not disable the consumer before running the migration script, data between inventory and SpiceDB may not be consistent. 

Disable the consumer in the inventory api config yaml. You may need to roll the pods for the change to take effect.
```shell
  consumer:
    enabled: false
```

With our zed context and gabi config set up to stage and port-forwarding into the SpiceDB service, we can execute our [migration script](/scripts/manual_migration_to_spicedb.py). Move the migration script to the same location as your `kafkadump.txt` file.

To get started, you can execute a dry run of the changes before making live changes to things.

Run
```shell
DRY_RUN=true python3 manual_migration_to_spicedb.py kafkadump.txt
```

If the changes look good and you're ready to go, execute without the `DRY_RUN` variable or set `DRY_RUN=false`

Run 
```shell
python3 manual_migration_to_spicedb.py kafkadump.txt
```

Voila! Your inventory DB data should now be consistent with SpiceDB after reprocessing the failing messages!

You can now re-enable the consumer so messages can begin processing. You may need to roll the pods for the change to take effect.

```shell
  consumer:
    enabled: true
```