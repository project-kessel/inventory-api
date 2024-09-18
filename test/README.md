# E2E test

## Inventory with Kafka

`make inventory-up-kafka`

## Build inventory
`make build`

## Run inventory locally
`./bin/inventory-api serve --config ./kafka-inventory-api.yaml`

## Run
`export INV_GRPC_URL=localhost:9081`
`export KAFKA_BOOTSTRAP_SERVERS=localhost:9092`
`go test ./... -count=1`

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