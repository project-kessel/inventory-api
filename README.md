# Common Inventory
This repository implements a common inventory system with eventing.


## Setup
```bash
make init
```
## API Changes (check against buf repository)
`make api`

## API Breaking Changes
`make api_breaking`

## Build
`make build`

## Build Container Images
By default the quay repository is `quay.io/cloudservices/kessel-inventory`. If you wish to use another for testing, set IMAGE value first
```shell
export IMAGE=your-quay-repo # if desired
make docker-build-push
```

## Run inventory api locally
### Run migration
`make migrate`
### Run service
`make run`


## Run docker-compose to setup
```make inventory-up``` to setup inventory-api, relations-api, spicedb, postgres

## Tear down docker-compose
`make inventory-down`


## Example Usage

### Health check endpoints

The inventory API includes health check endpoints for readiness and liveness probes.

#### Readyz
The readyz endpoint checks if the service is ready to handle requests.
```bash
curl http://localhost:8081/api/inventory/v1/readyz
```

#### Livez
The livez endpoint checks if the service is alive and functioning correctly.
```bash
curl http://localhost:8081/api/inventory/v1/livez
```

### Add hosts to inventory
To add hosts to the inventory, use the following `curl` command:

```bash
curl -H "Content-Type: application/json" --data "@data/host.json" http://localhost:8081/api/inventory/v1beta1/resources/rhel-hosts
```

Depending on the config file you're using, the curl command will require additional headers for authorization of the request.

### Adding a new relationship (k8s-policy is propagated to k8s-cluster)

To add a k8s-policy_ispropagatedto-k8s-cluster relationship you can use the following `curl` command:

```bash
curl -H "Content-Type: application/json" --data "@data/k8spolicy_ispropagatedto_k8scluster.json" http://localhost:8081/api/inventory/v1beta1/resource-relationships/k8s-policy_is-propagated-to_k8s-cluster
```

To update it, use the `PUT` verb as follows:

```bash
curl -X PUT -H "Content-Type: application/json" --data "@data/k8spolicy_ispropagatedto_k8scluster.json" http://localhost:8081/api/inventory/v1beta1/resource-relationships/k8s-policy_is-propagated-to_k8s-cluster
```

And finally, to delete it, use the `DELETE` verb, notice that the data file is different this time. We only need the reporter data to delete a relationship.

```bash
curl -X DELETE -H "Content-Type: application/json" --data "@data/relationship_reporter_data.json" http://localhost:8081/api/inventory/v1beta1/resource-relationships/k8s-policy_is-propagated-to_k8s-cluster
```

### Running with `make run`

We are using the included `.inventory-api.yaml` file which allows guest access.
Guest access currently [makes use](https://github.com/project-kessel/inventory-api/blob/main/internal/authn/guest/guest.go#L20) of the `user-agent` header to
populate the Identity header.

[data/host.json](./data/host.json) uses the `reporter_id: user@example.com`, hence you will need the following command:

```bash
curl -H "Content-Type: application/json" --user-agent user@example.com --data "@data/host.json" http://localhost:8081/api/inventory/v1beta1/resources/rhel-hosts
```

### Running with `make inventory-up`

This provides a [PSK file](https://github.com/project-kessel/inventory-api/blob/main/config/psks.yaml#L1) with a token "1234".
The default port in this setup are `8081` (http) and `9091`.

The following command will add the host to the inventory:

```bash
curl -H "Content-Type: application/json" -H "Authorization: bearer 1234" --data "@data/host.json" http://localhost:8081/api/inventory/v1beta1/resources/rhel-hosts
```

## Contribution
`make pr-check`


## Running Inventory api with sso (keycloak) docker compose setup
`make inventory-up-sso`

* Set up a keycloak instance running at port 8084 with [myrealm](myrealm.json)
* Set up a default service account with clientId: `test-svc` and password. Refer [get-token](scripts/get-token.sh)
* Refer [sso-inventory-api.yaml](sso-inventory-api.yaml) for configuration
* Refer [docker-compose-sso.yaml](docker-compose-sso.yaml) for docker-compose

Use service account user as `reporter_instance_id`
```
"reporter_instance_id": "service-account-svc-test"
```
Refer [host-service-account.json](data/host-service-account.json)

### Generate a sso token
`make get-token`

Export the token generated
`export TOKEN=`

Sample request with the authorization header

`curl -H "Authorization: bearer ${TOKEN}"  -H "Content-Type: application/json" --data "@data/host-service-account.json" http://localhost:8081/api/inventory/v1beta1/resources/rhel-hosts`

## Running Inventory api with kafka
Starts a local strimzi kafka and zookeeper:
```bash
make inventory-up-kafka
```

Start `inventory-api` using the `./kafka-inventory-api.yaml` config.
```bash
./bin/inventory-api serve --config ./kafka-inventory-api.yaml
```

In a separate terminal exec into the kafka pod so you can watch messages.
```bash
source ./scripts/check_docker_podman.sh
KAFKA_CONTAINER_NAME=$(${DOCKER} ps | grep inventory-api-kafka | awk '{print $1}')
${DOCKER} exec -i -t ${KAFKA_CONTAINER_NAME} /bin/bash
```

Start consuming messages in the pod.
```bash
./bin/kafka-console-consumer.sh --bootstrap-server localhost:9092 --topic kessel-inventory
```

In a separate terminal, post a resource to `inventory-api`:
```bash
curl -H "Content-Type: application/json" -H "Authorization: bearer 1234" --data "@data/k8s-cluster.json" http://localhost:8081/api/inventory/v1beta1/resources/k8s-clusters
```

Manually stop the `inventory-api` and then run `make inventory-down-kafka`
## Enable integration with Kessel relations API
Update the .inventory-api.yaml or inventory-api-compose.yaml

```yaml

authz:
  impl: kessel
  kessel:
    insecure-client: true
    url: localhost:9000
    enable-oidc-auth: false
```
Enable oidc authentication with sso

```yaml
authz:
  impl: kessel
  kessel:
    insecure-client: true
    url: localhost:9000
    enable-oidc-auth: true
    sa-client-id: "svc-test"
    sa-client-secret: "<secret>"
    sso-token-endpoint: "http://localhost:8084/realms/redhat-external/protocol/openid-connect/token"
```
## Debugging Inventory API using Vscode
Follow the [DEBUG](./DEBUG.md) guide
