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
- Go 1.23.6
- Make

### Debugging

See [DEBUG](./DEBUG.md) for instructions on how to debug

### Running locally

When running locally, the [default settings](./.inventory-api.yaml) file is used. By default, this configuration does the following:
- Exposes the inventory API in `localhost` and using port `8000` for http and port `9000` for grpc.
- Sets authentication mechanism to `allow-unauthenticated`, allowing users to be authenticated with their user-agent value.
- Sets authorization mechanism to `allow-all`.
- Sets database implementation to sqlite3 and the database file to `inventory.db`
- Sets the Inventory Consumer service to disabled
- Configures log level to `INFO`.

NOTE: You can update the [default settings](./.inventory-api.yaml) file as required to test different scenarios. Refer to the command line help (`make run-help`) or leverage one of the many pre-defined Docker Compose Test Setups
for information on the different parameters.

1. Clone the repository and navigate to the directory.
2. Install the required dependencies
    ```shell
    make init
    ```

3. Build the project
    ```shell
    # when building locally, use the local-build option as FIPS_ENABLED is set by default for builds
    make local-build
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

Due to various alternatives to running some images, we accept some arguments to override certain tools

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

> [!IMPORTANT]
> Note: The `podman-compose` provider struggles with compose files that leverage `depends_on` as it can't properly handle the dependency graphs. You can fix this issue on Linux by installing the `docker-compose-plugin` or also having `docker-compose` installed. When installed, podman uses the `docker-compose` provider by default instead. The benefit of the `docker-compose-plugin` is that it doesn't require the full Docker setup or Docker daemon!


### Running Locally using Docker Compose

Testing locally is fine for simple changes but in order to test the full application, it requires all the dependent backing services.

The Full Setup option for Docker Compose:
- Exposes the inventory API in `localhost` and using port `8000` for http and port `9000` for grpc.
- Sets both AuthN and AuthZ to Allow
- Deploys and configures Inventory to leverage postgres
- Deploys and configures Kafka, Zookeeper and Kafka Connect with Debezium configured for the Outbox table
- Enables and configures the Inventory Consumer for the local Kafka cluster
- Configures Inventory API using the [Full-Setup](development/configs/full-setup.yaml) config file

This setup allows testing the full inventory stack, but does not require Relations API. Calls that would get made to Relations API are just logged by the consumer.

To start with Full Setup configuration:
```shell
make inventory-up
```

To stop:
```shell
make inventory-down
```

### Alternative ways of running this service

There are numerous ways to run Inventory API using Docker Compose for various testing and debugging needs. For more options, see our [guide](./docs/dev-guides/docker-compose-options.md)

### Running in Ephemeral Cluster with Relations API using Bonfire

See [Testing Inventory in Ephemeral](./docs/ephemeral-testing.md) for instructions on how to deploy Kessel Inventory in the ephemeral cluster.

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

> Note: When testing in Stage, the current schema leveraged by Relations only supports notifications integrations and not any of the infra we have in our API (RHEL hosts, K8s Clusters, etc). Testing with any other resource type will throw errors from Relations API but will still succeed in Inventory API

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
- `notifications-integration`
- `k8s-cluster`
- `k8s-policy`

To add a rhel-host to the inventory:

To hit the REST endpoint use the following `curl` command

```shell
curl -H "Content-Type: application/json" --data "@data/testData/v1beta1/host.json" http://localhost:8000/api/inventory/v1beta1/resources/rhel-hosts
```

To hit the gRPC endpoint use the following `grpcurl` command

```
grpcurl -plaintext -d @ localhost:9000 kessel.inventory.v1beta1.resources.KesselRhelHostService.CreateRhelHost < data/testData/v1beta1/host.json
```

To update it:

To hit the REST endpoint

```shell
curl -XPUT -H "Content-Type: application/json" --data "@data/testData/v1beta1/host.json" http://localhost:8000/api/inventory/v1beta1/resources/rhel-hosts
```

To hit the gRPC endpoint

```
grpcurl -plaintext -d @ localhost:9000 kessel.inventory.v1beta1.resources.KesselRhelHostService.UpdateRhelHost < data/testData/v1beta1/host.json
```


and finally, to delete it, note that we use a different file, as the only required information is the reporter data.

To hit the REST endpoint

```shell
curl -XDELETE -H "Content-Type: application/json" --data "@data/testData/v1beta1/host-reporter.json" http://localhost:8000/api/inventory/v1beta1/resources/rhel-hosts
```

To hit the gRPC endpoint

```
grpcurl -plaintext -d @ localhost:9000 kessel.inventory.v1beta1.resources.KesselRhelHostService.DeleteRhelHost < data/testData/v1beta1/host-reporter.json
```
To add a notifications integration (useful for testing in stage)

```shell
# create the integration (auth is required for stage -- see internal docs)
curl -H "Content-Type: application/json" -H "Authorization: Bearer $TOKEN" -d @data/testData/v1beta1/notifications-integrations.json localhost:8000/api/inventory/v1beta1/resources/notifications-integrations

# delete the integration
curl -X DELETE -H "Content-Type: application/json" -H "Authorization: Bearer $TOKEN" -d @data/testData/v1beta1/notifications-integration-reporter.json localhost:8000/api/inventory/v1beta1/resources/notifications-integrations

```
### Adding a new relationship (k8s-policy is propagated to k8s-cluster)

To add a `k8s-policy_ispropagatedto-k8s-cluster` relationship, first lets add the related resources `k8s-policy` and `k8s-cluster`.

```shell
curl -H "Content-Type: application/json" --data "@data/testData/v1beta1/k8s-cluster.json" http://localhost:8000/api/inventory/v1beta1/resources/k8s-clusters
curl -H "Content-Type: application/json" --data "@data/testData/v1beta1/k8s-policy.json" http://localhost:8000/api/inventory/v1beta1/resources/k8s-policies
```

And then you can create the relation:

```shell
curl -H "Content-Type: application/json" --data "@data/testData/v1beta1/k8spolicy_ispropagatedto_k8scluster.json" http://localhost:8000/api/inventory/v1beta1/resource-relationships/k8s-policy_is-propagated-to_k8s-cluster
```

To update it:

```shell
curl -X PUT -H "Content-Type: application/json" --data "@data/testData/v1beta1/k8spolicy_ispropagatedto_k8scluster.json" http://localhost:8000/api/inventory/v1beta1/resource-relationships/k8s-policy_is-propagated-to_k8s-cluster
```

And finally, to delete it, notice that the data file is different this time. We only need the reporter data.

```shell
curl -X DELETE -H "Content-Type: application/json" --data "@data/testData/v1beta1/relationship_reporter_data.json" http://localhost:8000/api/inventory/v1beta1/resource-relationships/k8s-policy_is-propagated-to_k8s-cluster
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

For end-to-test info see [here](./test/README.md).

## Validating FIPS

Inventory API is configured to build with FIPS capable libraries and produce FIPS capaable binaries when running on FIPS enabled clusters.

To validate the current running container is FIPS capable:

```shell
# exec or rsh into running pod
# Reference the fips_enabled file that ubi9 creates for the host
cat /proc/sys/crypto/fips_enabled
# Expected output:
1

# Check go tool for the binary
go tool nm /usr/local/bin/inventory-api | grep FIPS
# Expected output should reference openssl FIPS settings

# Ensure openssl providers have a FIPS provider active
openssl list -providers | grep -A 3 fips
# Expected output
  fips
    name: Red Hat Enterprise Linux 9 - OpenSSL FIPS Provider
    version: 3.0.7-395c1a240fbfffd8
    status: active
```

## Contributing

Follow the steps below to contribute:

- Fork the project
- Create a new branch for your feature
- Run tests and Pr check
- Submit a pull request
