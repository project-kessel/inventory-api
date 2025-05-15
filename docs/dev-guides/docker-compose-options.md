# Alternative ways of running this service

## Local Kessel Inventory + Kessel Relations using built binaries

Inventory and Relations can also be run locally using built binaries, but the default config for Inventory will conflict with Relations.

To run Relations locally, see the [Relations README](https://github.com/project-kessel/relations-api?tab=readme-ov-file#prerequisites)

Relations will also require SpiceDB, this can be run using Podman/Docker (See relevant section also in the [Relations README](https://github.com/project-kessel/relations-api?tab=readme-ov-file#spicedb-using-dockerpodman))

For Inventory, an alternate config is available, pre-configured to expect a local running Relations API
```shell
# Setup
make local-build
make migrate

# run with the relations friendly config file
./bin/inventory-api serve --config development/configs/local-w-relations.yaml
```
> NOTE: The below setups all involve spinning up Kafka infrastructure with configuration jobs that run after. It can take about to a minute before the full Kafka stack is ready to go.

## Kessel Inventory + Kessel Relations using Docker Compose

A Relations-ready version of the Full Setup configuration exists that can be easily used with Relations API.

The only notable differences being:
- Inventory is configured to use ports `8081`, and `9081` to not conflict with Relations API
- Configures Inventory API using the [Full-Setup-Relations-Ready](development/configs/full-setup-relations-ready.yaml) config file

To start the Relations-ready version of Inventory:
```shell
make inventory-up-relations-ready
```

To deploy Relations API, it's recommended to clone the Relations API repo locally and leverage their existing [Docker Compose process](https://github.com/project-kessel/relations-api/tree/main?tab=readme-ov-file#spicedb-using-dockerpodman) to spin up the Relations API.

Both Inventory and Relations compose files are configured to use the same Docker network (`kessel`) to ensure network connectivity between all containers.

To stop Inventory:
```shell
make inventory-down
```

## Kessel Inventory + Kessel Relations w Monitoring Stack using Docker Compose

Useful for testing metrics, alerts, and dashboards locally where you can generate enough data and see it reflected in our Monitoring stack outside of Stage

This setup uses the same setup as the previous one but includes Prometheus, Grafana, and Alertmanager. It will require Relations API to be running to ensure consumer metrics are captured. Grafana is also pre-loaded with the local prometheus data source and loads our current dashboards which were extracted from our [dashboards folder](../../dashboards/)

To start Inventory and the monitoring stack:
```shell
make inventory-up-w-monitoring
```

To deploy Relations API, it's recommended to clone the Relations API repo locally and leverage their existing [Docker Compose process](https://github.com/project-kessel/relations-api/tree/main?tab=readme-ov-file#spicedb-using-dockerpodman) to spin up the Relations API.

Both Inventory and Relations compose files are configured to use the same Docker network (`kessel`) to ensure network connectivity between all containers.

To stop Inventory and the monitoring stack:
```shell
make inventory-down
```

> Note: If it's your first time spinning up Grafana, there is an initial login configured that you'll need to reset.
> username: `admin`
> password: `admin`.
> You will be prompted to reset it afterwards.
> The local running Prometheus is configured by default as a datasource so no other work is needed other than adding your own dashboards

Grafana URL: http://localhost:3000

Prometheus URL: http://localhost:9050

Alertmanager URL: http://localhost:9093

#### Testing Dashboard changes

If the dashboards in the dashboards folder have been updated, you can update the json files pre-loaded by Grafana with the `make update-local-dashboards` command. For dashboard updates and testing, it's recommended to update the dashboards in AppSRE Stage Grafana, capture the changes into the ConfigMaps in dashboard directory, then use the `make update-local-dashboards` commands to extract the json. This is a great way to test locally where you can easily hit the API as much as you want to generate data and see it in Grafana


## Local Kessel Inventory + Docker Compose Infra (Split Setup)

The Split Setup involves using a locally running Inventory API, but all other infra (Postgres, Kafka, etc) are deployed via Docker. This setup is great for debugging the local running binary but still have all the dependent services to test the full application.

To start the Split Setup:
```shell
make inventory-up-split
```

Then to run Inventory:
```shell
# Setup
make local-build

# run with the split config file (postgres host flag overwrites config locally which is set for Docker internal address)
./bin/inventory-api serve --config development/configs/split-setup.yaml --storage.postgres.host localhost
```

## Split Setup + Kessel Relations

Same as Split Setup which leverages a local Inventory API and Docker for the dependent infra, but updates the Inventory API server ports to not conflict with a running Relations API

To deploy Relations API, it's recommended to clone the Relations API repo locally and leverage their existing [Docker Compose process](https://github.com/project-kessel/relations-api/tree/main?tab=readme-ov-file#spicedb-using-dockerpodman) to spin up the Relations API.

To start the Relations-ready Split Setup:
```shell
make inventory-up-split-relations-ready
```

Then to run Inventory:
```shell
# Setup
make local-build

# run with the split config file (postgres host flag overwrites config locally which is set for Docker internal address)
./bin/inventory-api serve --config development/configs/split-setup-relations-ready.yaml --storage.postgres.host localhost
```

## Kessel Inventory + Kessel Relations + SSO (Keycloak) using Docker Compose

This setup expands on the Relations-ready Full Setup by:
- Setting up a Keycloak instance running at port 8084 with [myrealm](development/configs/myrealm.json) config file.
- Setting up a default service account with clientId: `test-svc`. Refer to [get-token](scripts/get-token.sh) to learn how to fetch a token.
- Configures Inventory API using the [Full-Setup-w-SSO](development/configs/full-setup-w-sso.yaml) config file

As before you'll need to run the Relations Compose steps available in the [Relations API repo](https://github.com/project-kessel/relations-api/tree/main?tab=readme-ov-file#spicedb-using-dockerpodman)

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
make inventory-down
```
