# Kessel Debug Container

The Kessel Debug Container is a useful tool for investigating Kessel service issues in cluster. It contains all the necessary CLI's needed for interacting with Kessel services, and configures environment variables for all Kessel endpoints and configurations needed to connect with dependent services such as SpiceDB or Kafka

**What's Included?**:
* Basic Networking tools (DNS testing tools, netcat, openssl, curl, wget, grpcurl)
* JQ for parsing JSON data
* Zed CLI to interact with SpiceDB
* Kafka command-line tools (available under `/opt/kafka/bin`)
* Helper scripts for basic tasks as part of our runbooks
* Kcat CLI for more advanced Kafka operations (allows setting headers when needed)

### Environment Configuration

The debug container is configured with environment variables consisting of Kessel endpoints and credentials, loaded from secrets already deployed with Kessel services or through a ConfigMap deployed with the debug container itself.

**What's Configured?**:
* SpiceDB endpoint and token configuration (via `ZED_ENDPOINT`, `ZED_TOKEN`, and `ZED_INSECURE` variables)
* Inventory API Endpoints: (via `INVENTORY_API_GRPC_ENDPOINT` and `INVENTORY_API_HTTP_ENDPOINT` variables)
* Relations API Endpoints: (via `RELATIONS_API_GRPC_ENDPOINT` and `RELATIONS_API_HTTP_ENDPOINT` variables)
* Kafka Endpoints and auth info (configured by running `source /usr/local/bin/env-setup.sh` if needed)
* Inventory API DB Endpoints and auth info (configured by running `source /usr/local/bin/env-setup.sh` if needed)
* Clowder cdappconfig for Inventory API mounted to `/cdapp`


### Running the Debug Container

Everything needed for the debug container is either encompassed in the deployment file, or captured from existing data in the namespace. Simply deploy using `oc` cli and connect to the running container

**To Run**:

>[!IMPORTANT]
> In order to use Kessel Debug, you must have permissions to create and exec/rsh to a container in the target environment. If this is for a production issue -- See the [Breakglass Process](https://project-kessel.github.io/docs-internal-overlay/for-red-hatters/running-kessel/monitoring-kessel/breakglass-sop).

```bash
oc process --local \
    -f https://raw.githubusercontent.com/project-kessel/inventory-api/refs/heads/main/tools/kessel-debug-container/kessel-debug-deploy.yaml \
    -p ENV=<target-environment (stage/prod)> | oc apply -f -
```

### Using the Debug Container

**To Access**:

```shell
oc rsh kessel-debug
```

If needed, Kafka and Inventory API DB connection information can be setup after accessing the pod by sourcing the `env-setup.sh` script available:

```shell
source /usr/local/bin/env-setup.sh
```

Sourcing the script configures some extra environment variables to simplify access to Kafka and Postgres, including setting up a JAAS auth config file required to authenticate to Kafka brokers when auth is configured for the environment

#### Accessing SpiceDB with Zed

SpiceDB connection details are set during deploy using the `ZED_ENDPOINT`, `ZED_TOKEN`, and `ZED_INSECURE` environment variables. No extra work is needed, and basic zed commands can be run with no extra flags or context setting.

For example:
* `zed schema read` to read the current schema
* `zed relationship read hbi/host` to read all relationships for resource type `hbi/host`

>[!Note]
> Using contexts with zed requires permissions to write to a config file which are denied in a rootless container. Environment variables prevent the need to use contexts and should be used in place of them.

>[!Warning]
> The zed cli is called using a wrapper script to prevent delete/write operations by default. It is possible to call the zed cli outside of the wrapper script if needed but write/delete operations should NOT be done lightly. These operations should not be done without Fabric Kessel team guidance.


#### Accessing Kafka

All Kafka CLI tools shipped with Kafka have been installed to `/opt/kafka/bin`, the pod starts in the `/opt/kafka` directory to shorten the path to run those tools. When leveraging any Kafka command-line tool in this path, at minimum the `--bootstrap-server` and `--command-config` flags will need to be set. The `BOOTSTRAP_SERVERS` env var can be used for `--bootstrap-server` value, and the `KAFKA_AUTH_CONFIG` env var can be used for the `--command-config` value. Note some commands have different names for the auth config flag (for example, `kafka-console-consumer` uses `--consumer.config`). Check the commands `--help` flag for details on what flag to use.

#### Accessing Inventory API DB

Postgres connection details are set using standard supported [Postgres environment variables](https://www.postgresql.org/docs/16/libpq-envars.html). To connect to Inventory API DB and execute queries, simply run `psql` to be dropped into a shell where you can execute queries against the DB.

#### Authenticating with kcat

`kcat` is a useful tool for both consuming events from topics and producing events to them, especially where headers must be defined, as the built-in Kafka tools do not support setting headers. We leverage `kcat` for pushing an event to a topic for replication fixes or for trigging ad-hoc snapshots with Debeizum.

Similar to using the Kafka CLI tools, any `kcat` commands will require the bootstrap server address(es) and authentication information. The following can be affixed to any kcat commands to ensure proper connection and authentication:

`-b $BOOTSTRAP_SERVERS -X security.protocol=sasl_ssl -X sasl.mechanisms=SCRAM-SHA-512 -X sasl.username=$KAFKA_USERNAME -X sasl.password=$KAFKA_PW`

### Removing the Debug Container

Its critical to always remove the debug container when finished.

To destroy the container:

```bash
oc process --local \
    -f https://raw.githubusercontent.com/project-kessel/inventory-api/refs/heads/main/tools/kessel-debug-container/kessel-debug-deploy.yaml \
    -p ENV=<target-environment (stage/prod)> | oc delete -f -
```
