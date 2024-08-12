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


## Running Inventory api with sso (keycloak) docker compose setup
`make inventory-up-sso`

* Set up a keycloak instance running at port 8084 with [myrealm](myrealm.json)
* Set up a default service account with clientId: `test-svc` and password. Refer [get-token](scripts/get-token.sh)
* Refer [sso-invetory-api.yaml](sso-inventory-api.yaml) for configuration
* Refer [docker-compose-sso.yaml](docker-compose-sso.yaml) for docker-compose

Use service account user as `reporter_instance_id`
```
"reporter_instance_id": "service-account-svc-test"
```
Refer [host-service-account.json](data/host-service-account.json)

### Generate a sso token
`make get-token`
