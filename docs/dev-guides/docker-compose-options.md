# Local Development with Docker Compose

## Quick Reference

| Make Target | What It Starts | Use Case |
|---|---|---|
| `make kessel-up` | Full Kessel suite (all services, pre-built images) | Most common -- test the full stack |
| `make kessel-up-monitoring` | Full Kessel + Prometheus/Grafana/Alertmanager | Metrics and dashboard development |
| `make inventory-up` | Inventory API only (built from source) | Developing Inventory API code |
| `make inventory-up-relations-ready` | Inventory API (built from source, x-rh-identity auth) | Testing with external Relations API |
| `make inventory-up-w-monitoring` | Inventory API + monitoring stack | Inventory metrics testing |
| `make inventory-up-sso` | Inventory API + Keycloak SSO | Testing OIDC authentication |
| `make inventory-up-split` | Infra only (Postgres, Kafka, Connect) | Debugging with local binary + debugger |
| `make inventory-up-split-relations-ready` | Infra only (relations-compatible ports) | Debugging with local binary + Relations |
| `make monitoring-only` | Prometheus/Grafana/Alertmanager for ephemeral | Monitoring an ephemeral namespace |

All setups are stopped with `make inventory-down` except Full Kessel which uses `make kessel-down`.

> NOTE: Setups that include Kafka infrastructure can take about a minute before the full Kafka stack is ready.

---

## Full Kessel Stack (Recommended)

Deploy the entire Kessel suite locally with a single command: Inventory API, Relations API + SpiceDB, Inventory Consumer, Kafka, and Kafka Connect. Uses pre-built images so no other repos need to be cloned.

```shell
make kessel-up
```

This starts all services using the compose file at `development/full-kessel/docker-compose.yaml`. The SpiceDB schema is automatically downloaded from the [stage rbac-config repo](https://raw.githubusercontent.com/RedHatInsights/rbac-config/refs/heads/master/configs/stage/schemas/schema.zed) at startup.

> NOTE: If you previously ran `make db/setup` for the Quick Start workflow, tear it down first with `make db/teardown` — the standalone postgres container holds port 5435 which conflicts with the HBI database in the full-kessel stack.

### Ports

| Port | Service |
|------|---------|
| 8081 | Inventory API (HTTP) |
| 9081 | Inventory API (gRPC) |
| 8000 | Relations API (HTTP) |
| 9000 | Relations API (gRPC) |
| 50051 | SpiceDB (gRPC) |
| 8083 | Inventory Kafka Connect |
| 8084 | KIC Kafka Connect |
| 9092 | Kafka |
| 5432 | Relations DB (postgres) |
| 5433 | Inventory DB (postgres) |
| 5435 | HBI DB (postgres) |
| 9050 | Prometheus (monitoring profile) |
| 9093 | Alertmanager (monitoring profile) |
| 3000 | Grafana (monitoring profile) |

### Overriding Images

Edit `development/full-kessel/.env` to use custom images for development:

```env
INVENTORY_API_IMAGE=localhost/kessel-inventory:dev
RELATIONS_API_IMAGE=quay.io/redhat-services-prod/project-kessel-tenant/kessel-relations/relations-api:latest
INVENTORY_CONSUMER_IMAGE=quay.io/redhat-services-prod/project-kessel-tenant/kessel-inventory-consumer/inventory-consumer:latest
```

### Using a Custom SpiceDB Schema

To use a local schema file instead of downloading from GitHub:

```env
SCHEMA_ZED_FILE=/path/to/your/schema.zed
```

Or change the download URL:

```env
SCHEMA_ZED_URL=https://example.com/your-schema.zed
```

### With Monitoring Stack

To include Prometheus, Grafana, and Alertmanager:

```shell
make kessel-up-monitoring
```

Grafana is pre-loaded with a local Prometheus datasource and the current dashboards from the [dashboards folder](../../dashboards/). Prometheus is configured to scrape all Kessel services (Inventory API, Relations API, Consumer, and both Kafka Connect instances).

See [Monitoring Info](#monitoring-info) for URLs and login details.

### Teardown

```shell
make kessel-down
```

---

## Inventory API Only (Built from Source)

These setups use the compose file at `development/docker-compose.yaml` and build Inventory API from source. They do **not** include Relations API, SpiceDB, or Inventory Consumer -- those must be run separately if needed.

All of these are stopped with `make inventory-down`.

### Basic Setup

Deploys Inventory API with Postgres, Kafka, Zookeeper, and Kafka Connect with Debezium. AuthN and AuthZ are set to allow-all.

```shell
make inventory-up
```

Inventory API is available at `localhost:8000` (HTTP) and `localhost:9000` (gRPC).

### With Relations API (x-rh-identity Auth)

Same as above but on ports `8081`/`9081` to avoid conflicts with Relations API, using x-rh-identity chain authentication and kessel authorization.

```shell
make inventory-up-relations-ready
```

To deploy Relations API, clone the [Relations API repo](https://github.com/project-kessel/relations-api) and use their [Docker Compose process](https://github.com/project-kessel/relations-api/tree/main?tab=readme-ov-file#spicedb-using-dockerpodman). Both compose files share the `kessel` Docker network for connectivity.

### With Monitoring Stack

Same as basic setup (allow-unauthenticated auth, kessel authz) plus Prometheus, Grafana, and Alertmanager:

```shell
make inventory-up-w-monitoring
```

See [Monitoring Info](#monitoring-info) for URLs and login details.

#### Testing Dashboard Changes

If dashboards have been updated, refresh the local Grafana copies with `make update-local-dashboards`. For dashboard development, the recommended workflow is: update dashboards in AppSRE Stage Grafana, capture changes into the ConfigMaps in the dashboard directory, then use `make update-local-dashboards` to extract the JSON for local testing.

### With SSO (Keycloak)

Adds a Keycloak instance with OIDC authentication:
- Keycloak at port 8084 with [myrealm](../../development/configs/myrealm.json) config
- Default service account with clientId: `test-svc`
- Configures Inventory API using the [authn-sso](../../development/configs/authn-sso.yaml) config file

```shell
make inventory-up-sso
```

To get a token and use it:
```shell
make get-token
export TOKEN=<output>
curl -H "Authorization: bearer ${TOKEN}" http://localhost:8081/api/kessel/v1/livez
```

---

## Split Setup (Local Binary + Docker Infra)

Run Inventory API as a local binary (great for debugging with `dlv` or your IDE) while Docker handles all backing services (Postgres, Kafka, Connect).

### Without Relations

```shell
make inventory-up-split
make local-build

./bin/inventory-api serve --config development/configs/base.yaml \
  --storage.postgres.host localhost \
  --consumer.bootstrap-servers localhost:9092 \
  --authz.impl allow-all
```

### With Relations

Uses ports `8081`/`9081` to avoid conflicts with a locally running Relations API:

```shell
make inventory-up-split-relations-ready
make local-build

./bin/inventory-api serve --config development/configs/base.yaml \
  --storage.postgres.host localhost \
  --consumer.bootstrap-servers localhost:9092 \
  --authz.kessel.url localhost:9000
```

To deploy Relations API, clone the [Relations API repo](https://github.com/project-kessel/relations-api) and use their [Docker Compose process](https://github.com/project-kessel/relations-api/tree/main?tab=readme-ov-file#spicedb-using-dockerpodman).

---

## Local Binaries (No Docker)

Inventory and Relations can both be run as local binaries, but the default config for Inventory will conflict with Relations.

To run Relations locally, see the [Relations README](https://github.com/project-kessel/relations-api?tab=readme-ov-file#prerequisites). Relations also requires SpiceDB ([instructions](https://github.com/project-kessel/relations-api?tab=readme-ov-file#spicedb-using-dockerpodman)).

For Inventory, use the relations-compatible config:
```shell
make local-build
make migrate

./bin/inventory-api serve --config development/configs/local-w-relations.yaml
```

---

## Monitoring Stack Only (Ephemeral)

Runs Prometheus, Grafana, and Alertmanager locally, configured to scrape services in an ephemeral namespace. Useful for monitoring Kessel services deployed via bonfire without running them locally.

**Prerequisites:** Deploy Kessel to ephemeral or target an existing namespace:
```shell
bonfire deploy kessel -C kessel-inventory
# OR
oc project existing-ephemeral-namespace
```

```shell
make monitoring-only   # start
make monitoring-down   # stop
```

> Note: Grafana is configured with a `prometheus-ephem` datasource. Make sure to select it on any dashboards.

See [Monitoring Info](#monitoring-info) for URLs and login details.

---

## Monitoring Info

Applies to all setups that include the monitoring stack.

| Service | URL |
|---|---|
| Grafana | http://localhost:3000 |
| Prometheus | http://localhost:9050 |
| Alertmanager | http://localhost:9093 |

> Grafana default login: username `admin`, password `admin`. You will be prompted to change it on first login.
