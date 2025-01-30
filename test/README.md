# E2E test

## Run all E2E tests with Kind

```shell
make inventory-up-kind
```

Includes:

* Builds inventory container image
* Build E2E test container image
* Deploys local kind cluster with contour, strimzi, postgres, spicedb, kafka, relations-api and inventory
* Runs E2E test job in kind cluster

Check the result of the E2E test job with:

```shell
make check-e2e-tests
```

Teardown the kind cluster with:

```shell
make inventory-down-kind
```

### Run E2E test job in cluster

```shell
kubectl apply -f e2e-batch.yaml
```

Note: this is done already by `make inventory-up-kind`

## Inventory with Kafka

`make inventory-up-kafka`

## Build inventory

`make build`

## Run Inventory GRPC test with Kafka consumer

update kafka-inventory-api.yaml

```yaml
authn:
  psk:
    pre-shared-key-file: ./config/psks.yaml
```

### Run inventory locally

`./bin/inventory-api serve --config ./kafka-inventory-api.yaml`

### Run Inventory GRPC test with Kafka

```export INV_GRPC_URL=localhost:9081```
```export KAFKA_BOOTSTRAP_SERVERS=localhost:9092```

```go test ./... -count=1 -skip 'TestInventoryAPIHTTP_*'```

## Run Inventory HTTP test with Kafka consumer

### Run inventory locally

`./bin/inventory-api serve --config ./kafka-inventory-api.yaml`

### Run Inventory HTTP test with Kafka

#### Kafka SSL client certs

```export KAFKA_SECURITY_PROTOCOL=SSL```
```export KAFKA_CLIENT_CERT=/<path>/client-tls.crt```
```export KAFKA_CLIENT_KEY=/<path>/client-tls.key```
```export KAFKA_SSL_KEY_PASSWORD"=if any...```

#### Default TLS is set false

```export INV_HTTP_URL=localhost:8081```
```export KAFKA_BOOTSTRAP_SERVERS=localhost:9092```

#### Enable TLS

```export INV_TLS_INSECURE=false```
```INV_TLS_CERT_FILE=/<path>/tls.crt```
```INV_TLS_KEY_FILE=/<path>/tls.key```
```INV_TLS_CA_FILE=/<path>/ca.pem```

Run test

```go test ./... -count=1 -skip 'TestInventoryAPIGRPC_*'```

## Run E2E tests in isolation

Build and run an e2e test image, pointed a custom test environment.

### Build E2E test container image

Run from the root directory

```shell
docker build -t localhost/inventory-e2e-tests:latest -f Dockerfile-e2e
```

### Run E2E test image using docker run

```shell
docker run -e INV_GRPC_URL=<host>:9081 -e KAFKA_BOOTSTRAP_SERVERS=<host:9092> localhost/inventory-e2e-tests:latest
```