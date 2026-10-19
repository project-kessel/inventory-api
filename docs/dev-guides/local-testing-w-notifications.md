# Running Notifications + Inventory using Local Built Binaries

This process goes through running Notifications and Inventory API locally using built binaries. Since there are database dependencies involved, podman/docker are leveraged to handle running databases.

## Prerequisites

You'll need the following tools:
* Docker/Podman
* make
* git
* [Maven](https://maven.apache.org/install.html)

You'll also need the following repos cloned to your system:
* [Inventory API](https://github.com/project-kessel/inventory-api)
* [Notifications Backend](https://github.com/RedHatInsights/notifications-backend)

### Running SpiceDB:

Inventory API requires SpiceDB for authorization. Start SpiceDB using the embedded setup:

```shell
cd /path/to/inventory-api
make inventory-up-spicedb
```

This starts SpiceDB with its backing Postgres, Kafka, and the Inventory Consumer automatically.

### Running Inventory:
1) Change to the Inventory API code path: `cd /path/to/inventory-api`

2) Build the Inventory API binary: `make build`

3) Create and setup the database:

```shell
# default DB is SQLite, removing the file ensures a new database is created each time
rm ./inventory.db
make migrate
```

4) Run Inventory API:

```shell
# Inventory API includes relations functionality and connects directly to SpiceDB
./bin/inventory-api serve --config development/configs/local-w-relations.yaml
```

### Running Notifications

1) Change to the Notifications Backend code path: `cd /path/to/notifications-backend`
2) Create the notifications DB using Podman/Docker

```shell
podman/docker run --name notifications_db --detach -e POSTGRES_USER=postgres -e POSTGRES_PASSWORD=postgres -e POSTGRES_DB=notifications -p 5432:5432 docker.io/postgres:latest -c log_statement=all
```

3) Use Maven to Clean, Compile, Test, and Install

```shell
cd common
mvn clean install
cd ..
./mvnw install

# OR to skip tests (they take about 15 mins)
./mvnw install -Dmaven.test.skip
```

4) Run the Notifications Service

```shell
# Inventory API now handles both inventory and relations — enable the inventory integration
./mvnw clean quarkus:dev -Dnotifications.use-default-template=true -Dnotifications.kessel-inventory.enabled=true -pl :notifications-backend
```


### Cleanup!

1) Shutdown Notifications: enter `q` in the running window to shut it down

2) Kill Inventory API: `ctrl+c` or use whatever fun killing technique you like!

3) Teardown SpiceDB and backing services:

```shell
cd /path/to/inventory-api
make inventory-down
```

4) Teardown Notifications DB: `podman/docker stop notifications_db && podman/docker rm notifications_db`
