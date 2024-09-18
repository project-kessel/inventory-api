# E2E test

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

#### Enabel tls
```export INV_TLS_INSECURE=false```
```INV_TLS_CERT_FILE=/<path>/tls.crt```
```INV_TLS_KEY_FILE=/<path>/tls.key```
```INV_TLS_CA_FILE=/<path>/ca.pem```
``

```go test ./... -count=1 -skip 'TestInventoryAPIGRPC_*'```

## Docker Build

Run from the root directory
```
docker build -t localhost/inventory-e2e-tests:latest -f Dockerfile-e2e
```

## Run e2e test locally on using docker run
```
docker run -e INV_GRPC_URL=<host>:9081 -e KAFKA_BOOTSTRAP_SERVERS=<host:9092> localhost/inventory-e2e-tests:latest
```

## Deploy e2e batch test yaml to Kind cluster
```kubectl apply -f e2e-batch.yaml```