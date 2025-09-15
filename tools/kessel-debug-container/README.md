# Kessel Debug Container

The Kessel Debug Container is a useful tool for investigating Kessel service issues in cluster. It contains all the necessary CLI's needed for interacting with Kessel services, and configures environment variables for all Kessel endpoints and configurations needed to connect with dependent services such as SpiceDB or Kafka

**What's Included?**:
* Basic Networking tools (DNS testing tools, netcat, openssl, curl, wget, grpcurl)
* JQ for parsing JSON data
* Zed CLI to interact with SpiceDB
* Kafka command-line tools (available under `/opt/kafka/bin`)
* Helper scripts for basic tasks as part of our runbooks

### Environment Configuration

The debug container is configured with environment variables consisting of Kessel endpoints and credentials, loaded from secrets already deployed with Kessel services or through a ConfigMap deployed with the debug container itself.

**What's Configured?**:
* SpiceDB endpoint and token configuration (via `ZED_ENDPOINT`, `ZED_TOKEN`, and `ZED_INSECURE` variables)
* Inventory API Endpoints: (via `INVENTORY_API_GRPC_ENDPOINT` and `INVENTORY_API_HTTP_ENDPOINT` variables)
* Relations API Endpoints: (via `RELATIONS_API_GRPC_ENDPOINT` and `RELATIONS_API_HTTP_ENDPOINT` variables)
* Kafka Endpoints and auth info (configured by running `source /usr/local/bin/env-setup.sh` if needed)
* Clowder cdappconfig for Inventory API mounted to `/cdapp`


### Running the Debug Container

Everything needed for the debug container is either encompassed in the deployment file, or captured from existing data in the namespace. Simply deploy using `oc` cli and connect to the running container

**To Run**:

```shell
oc process --local -f tools/kessel-debug-container/kessel-debug-deploy.yaml
    -p ENV=<target-environment (int, stage, or prod)> | oc apply -f -
```

### Using the Debug Container

**To Access**:

```shell
oc rsh $(oc get pod -l app=kessel-debug -o name)
```

If needed, Kafka connection information can be setup after accessing the pod by sourcing the `env-setup.sh` script available. This will export the Kafka bootstrap server address(es) to the `BOOTSTRAP_SERVERS` variable, as well as create a JAAS auth config file and set the path to it under the `KAFKA_AUTH_CONFIG` variable:

```shell
source /usr/local/bin/env-setup.sh
```

When leveraging any Kafka command-line tools, the `KAFKA_AUTH_CONFIG` can be provided to most commands using the `--command-config` flag to ensure authentication is used and avoid errors. Some commands have different names for this flag, see their respective `--help` commands for details.

### Removing the Debug Container

Its critical to always remove the debug container when finished.

To destroy the container:

```shell
oc process --local -f tools/kessel-debug-container/kessel-debug-deploy.yaml
    -p ENV=<target-environment (int, stage, or prod)> | oc delete -f -
```
