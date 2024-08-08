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

To add hosts to the inventory, use the following `curl` command:

```bash
curl -H "Content-Type: application/json" --data "@data/host.json" http://localhost:8081/api/inventory/v1beta1/rhelHosts
```

## Contribution
`make pr-check`

