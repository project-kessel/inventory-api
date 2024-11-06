# Common Inventory

This repository implements a common inventory system with eventing.

[![License](https://img.shields.io/badge/License-Apache_2.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)

## Table of Contents
- [Development Setup](#development-setup)
- [Example Usage](#example-usage)
- [Configuration](#configuration)
- [Testing](#testing)
- [Contributing](#contributing)

## Development Setup

### Prerequisites
- Go 1.23.1+
- Make

### Running locally

When running locally, (.inventory-api.yaml)[./.inventory-api.yaml] file is used. By default, this configuration does the following:
- Exposes the inventory API in `localhost` and using port `8000` for http and port `9000` for grpc.
- Sets authentication mechanism to `allow-unauthenticated`, allowing users to be authenticated with their user-agent value.
- Sets authorization mechanism to `allow-all`.
- Configures eventing mechanism to go to stdout.
- Sets database implementation to sqlite3 and the database file to `inventory.db`
- Configures log level to `INFO`.

NOTE: You can update the [default settings](./.inventory-api.yaml) file as required to test different scenarios. Refer to the command line help (`make run-help`)
for information on the different parameters.

1. Clone the repository and navigate to the directory.
2. Install the required dependencies
    ```shell
    make init
    ```

3. Build the project
    ```shell
    make build
    ```

4. Run the database migration
    ```shell
    make migrate
    ```

5. Start the development server
    ```shell
    make run
    ```

### Overriding commands in the Makefile

Due to various alternatives to running some images, we accept some arguments to override certains tools

#### GO binary

Since there are official instructions on how to [manage multiple installs](https://go.dev/doc/manage-install)
We accept the `GO` parameter when running make. e.g.

```shell
GO=go1.23.1 make run
```

or

```shell
export GO=go1.23.1
make run
```

#### Podman / Docker

We will use `podman` if it is installed, and we will fall back to `docker`. You can also specify if you want to ensure a particular binary is used
by providing `DOCKER` parameter e.g.

```shell
DOCKER=docker make api 
```

or

```shell
export DOCKER=docker
make api
```

### Debugging

See [DEBUG](./DEBUG.md) for instructions on how to debug
   
### Alternatives way of running this service

#### Kessel Inventory + Kessel Relations

There is Make target to run inventory-api (with postgres) and relations api (with spicedb).
It uses compose to build the current inventory code and spin up containers with the required dependecies.

- This provides a [PSK file](./config/psks.yaml#L1) with a token "1234".
- Default ports in this setup are `8081` for http and `9091` for grpc.
- Refer to [inventory-api-compose.yaml](./inventory-api-compose.yaml) for additional configuration

To start use:
```shell
make inventory-up
```

To stop use:

```shell
make inventory-down
```

#### Kessel-Inventory + Kafka

In order to use the kafka configuration, one has to run strimzi and zookeeper. 
You can do this by running;

```shell
make inventory-up-kafka
```

Start Kessel Inventory and configuring it to connect to kafka:
```yaml
eventing:
  eventer: kafka
  kafka:
    bootstrap-servers: "localhost:9092"
    # Adapt as required
    # security-protocol: "SASL_PLAINTEXT"
    # sasl-mechanism: PLAIN
```

You can use our default config with kafka by running:

```shell
INVENTORY_API_CONFIG="./kafka-inventory-api.yaml"  make run
```

- Refer to [kafka-inventory-api.yaml](./kafka-inventory-api.yaml) for additional configuration


Once started, you can watch the messages using [kcat](https://github.com/edenhill/kcat) (formerly known as kafkacat)
or by exec into the running container like this:

```shell
source ./scripts/check_docker_podman.sh
KAFKA_CONTAINER_NAME=$(${DOCKER} ps | grep inventory-api-kafka | awk '{print $1}')
${DOCKER} exec -i -t ${KAFKA_CONTAINER_NAME} /bin/bash

# Once in the container
./bin/kafka-console-consumer.sh --bootstrap-server localhost:9092 --topic kessel-inventory
```

Manually terminate Kessel inventory and then run the following to stop kafka:

```shell
make inventory-down-kafka
```

#### Kessel Inventory + Kessel Relations + Keycloak

Similar as above, but instead of running Kafka, this will configure inventory to use a Keycloak service for authentication.

- Sets up a keycloak instance running at port 8084 with [myrealm](myrealm.json) config file.
- Set up a default service account with clientId: `test-svc`. Refer to [get-token](scripts/get-token.sh) to learn how to fetch a token.
- Refer to [sso-inventory-api.yaml](./sso-inventory-api.yaml) for additional configuration

To start use:
```shell
make inventory-up-sso
```

Once it has started, you will need to fetch a token and use it when making calls to the service.

To get a token use:
```shell
make get-token
```

You can then export an ENV with that value and use in calls such as:

```shell
curl -H "Authorization: bearer ${TOKEN}" # ...
```

To stop use:

```shell
make inventory-down-sso
```

#### Running in Ephemeral Cluster with Relations API using Bonfire

Instructions to deploy Kessel Inventory in an ephemeral cluster can be found on [Kessel docs](https://cuddly-tribble-gq7r66v.pages.github.io/kessel/inventory-api/ephemeral/)

### API/Proto files Changes

Once there is any change in the `proto` files (under (/api/kessel)[./api/kessel]) an update is required.

This command will generate code and an (openapi)[./openapi.yaml] file from the `proto files`.
```shell
make api
```

We can run the following command to update if there are expected breaking changes.
```shell
make api_breaking
```

### Build Container Images

By default, the quay repository is `quay.io/cloudservices/kessel-inventory`. If you wish to use another for testing, set IMAGE value first
```shell
export IMAGE=your-quay-repo # if desired
make docker-build-push
```

### Build Container Images (macOS)
This is an alternative to the above command for macOS users, but should work for any arch
```shell
export QUAY_REPO_INVENTORY=your-quay-repo # required
podman login quay.io # required, this target assumes you are already logged in
make build-push-minimal
```

## Example Usage

All these examples use the  REST API and assume we are running the default local version
adjustments needs to be made to the curl requests if running  with different configuration, 
such as port, authentication mechanisms, etc.

### Health check endpoints

The Kessel Inventory includes health check endpoints for readiness and liveness probes.

#### Readyz
The readyz endpoint checks if the service is ready to handle requests.
```shell
curl http://localhost:8000/api/inventory/v1/readyz
```

#### Livez
The livez endpoint checks if the service is alive and functioning correctly.
```shell
curl http://localhost:8000/api/inventory/v1/livez
```

### Resource lifecycle

Resources can be added, updated and deleted to our inventory. Right now we support the following resources:
- `rhel-host`
- `integration`
- `k8s-cluster`
- `k8s-policy`

To add a rhel-host to the inventory, use the following `curl` command

```shell
curl -H "Content-Type: application/json" --data "@data/host.json" http://localhost:8000/api/inventory/v1beta1/resources/rhel-hosts
```

To update it:

```shell
curl -XPUT -H "Content-Type: application/json" --data "@data/host.json" http://localhost:8000/api/inventory/v1beta1/resources/rhel-hosts
```

and finally, to delete it, note that we use a different file, as the only required information is the reporter data.

```shell
curl -XDELETE -H "Content-Type: application/json" --data "@data/host-reporter.json" http://localhost:8000/api/inventory/v1beta1/resources/rhel-hosts
```

### Adding a new relationship (k8s-policy is propagated to k8s-cluster)

To add a `k8s-policy_ispropagatedto-k8s-cluster` relationship, first lets add the related resources `k8s-policy` and `k8s-cluster`.

```shell
curl -H "Content-Type: application/json" --data "@data/k8s-cluster.json" http://localhost:8000/api/inventory/v1beta1/resources/k8s-clusters
curl -H "Content-Type: application/json" --data "@data/k8s-policy.json" http://localhost:8000/api/inventory/v1beta1/resources/k8s-policies
```

And then you can create the relation:

```shell
curl -H "Content-Type: application/json" --data "@data/k8spolicy_ispropagatedto_k8scluster.json" http://localhost:8000/api/inventory/v1beta1/resource-relationships/k8s-policy_is-propagated-to_k8s-cluster
```

To update it:

```shell
curl -X PUT -H "Content-Type: application/json" --data "@data/k8spolicy_ispropagatedto_k8scluster.json" http://localhost:8000/api/inventory/v1beta1/resource-relationships/k8s-policy_is-propagated-to_k8s-cluster
```

And finally, to delete it, notice that the data file is different this time. We only need the reporter data.

```shell
curl -X DELETE -H "Content-Type: application/json" --data "@data/relationship_reporter_data.json" http://localhost:8000/api/inventory/v1beta1/resource-relationships/k8s-policy_is-propagated-to_k8s-cluster
```

## Configuration

### Enable integration with Kessel relations

The default development config has this option disabled. You can check [Alternatives way of running this service](#alternatives-way-of-running-this-service)
for configurations that have Kessel relations enabled. 

Supposing Kessel relations is running in `localhost:9000`, you can enable it by updating the config as follows:

```yaml
authz:
  impl: kessel
  kessel:
    insecure-client: true
    url: localhost:9000
    enable-oidc-auth: false
```

If you want to enable OIDC authentication with SSO, you can use this instead:

```yaml
authz:
  impl: kessel
  kessel:
    insecure-client: true
    url: localhost:9000
    enable-oidc-auth: true
    sa-client-id: "<service-id>"
    sa-client-secret: "<secret>"
    sso-token-endpoint: "http://localhost:8084/realms/redhat-external/protocol/openid-connect/token"

```

## Testing

Tests can be run using:

```shell
make test
```

## Contributing

Follow the steps below to contribute:

- Fork the project
- Create a new branch for your feature
- Submit a pull request
