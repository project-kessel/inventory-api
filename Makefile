FIPS_ENABLED?=true
ifeq ($(GO),)
GO:=$(shell command -v go)
endif

GOHOSTOS:=$(shell $(GO) env GOHOSTOS)
GOPATH:=$(shell $(GO) env GOPATH)
GOOS?=$(shell $(GO) env GOOS)
GOARCH?=$(shell $(GO) env GOARCH)
GOBIN?=$(shell $(GO) env GOBIN)
GOFLAGS_MOD ?=

GOENV=GOOS=${GOOS} GOARCH=${GOARCH} CGO_ENABLED=1 GOFLAGS="${GOFLAGS_MOD}"
GOBUILDFLAGS=-gcflags="all=-trimpath=${GOPATH}" -asmflags="all=-trimpath=${GOPATH}"

ifeq (${FIPS_ENABLED}, true)
GOFLAGS_MOD+=-tags=fips_enabled
GOFLAGS_MOD:=$(strip ${GOFLAGS_MOD})
GOENV+=GOEXPERIMENT=strictfipsruntime,boringcrypto
GOENV:=$(strip ${GOENV})
endif

IMAGE ?="quay.io/cloudservices/kessel-inventory"
IMAGE_TAG=$(git rev-parse --short=7 HEAD)
GIT_COMMIT=$(git rev-parse --short HEAD)

ifeq ($(DOCKER),)
DOCKER:=$(shell command -v podman || command -v docker)
endif

API_PROTO_FILES:=$(shell find api -name *.proto)

TITLE:="Kessel Asset Inventory API"
ifeq ($(VERSION),)
VERSION:=$(shell git describe --tags --always)
endif
INVENTORY_SCHEMA_VERSION=0.11.0

.PHONY: init
# init env
init:
	$(GO) install github.com/go-kratos/kratos/cmd/kratos/v2@latest
	$(GO) install github.com/google/wire/cmd/wire@latest
	$(GO) install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	$(GO) get google.golang.org/grpc/cmd/protoc-gen-go-grpc
	$(GO) install google.golang.org/grpc/cmd/protoc-gen-go-grpc
	$(GO) install github.com/go-kratos/kratos/cmd/protoc-gen-go-http/v2@latest
	$(GO) install github.com/google/gnostic/cmd/protoc-gen-openapi@latest
	$(GO) get github.com/envoyproxy/protoc-gen-validate
	$(GO) install github.com/envoyproxy/protoc-gen-validate

.PHONY: api
# generate api proto
api:
	@echo "Generating api protos"
	@$(DOCKER) build -t custom-protoc ./api
	@$(DOCKER) run -t --rm -v $(PWD)/api:/api:rw,z -v $(PWD)/openapi.yaml:/openapi.yaml:rw,z \
	-w=/api/ custom-protoc sh -c "buf generate && \
		buf lint && \
		buf breaking --against 'buf.build/project-kessel/inventory-api' "

.PHONY: api_breaking
# generate api proto
api_breaking:
	@echo "Generating api protos, allowing breaking changes"
	@$(DOCKER) build -t custom-protoc ./api
	@$(DOCKER) run -t --rm -v $(PWD)/api:/api:rw,z -v $(PWD)/openapi.yaml:/openapi.yaml:rw,z \
	-w=/api/ custom-protoc sh -c "buf generate && \
		buf lint"

# .PHONY: api
# # generate api proto
# api:
# 	@echo "Generating api protos"
# 	@$(DOCKER) build -t custom-protoc ./api
# 	@$(DOCKER) run -t --rm -v $(PWD)/api:/api:rw -v $(PWD)/openapi.yaml:/openapi.yaml:rw \
# 	-w=/api/ custom-protoc sh -c "buf generate && buf lint"

.PHONY: build
# build
build:
	$(warning Setting GOEXPERIMENT=strictfipsruntime,boringcrypto - this generally causes builds to fail unless building inside the provided Dockerfile. If building locally, run `make local-build`)
	mkdir -p bin/ && ${GOENV} GOOS=${GOOS} ${GO} build ${GOBUILDFLAGS} -ldflags "-X cmd.Version=$(VERSION)" -o ./bin/ ./...

.PHONY: local-build
# local-build to ensure FIPS is not enabled which would likely result in a failed build locally
local-build:
	mkdir -p bin/ && $(GO) build -ldflags "-X cmd.Version=$(VERSION)" -o ./bin/ ./...

.PHONY: docker-build-push
docker-build-push:
	./build_deploy.sh

.PHONY: build-push-minimal
build-push-minimal:
	./build_push_minimal.sh

.PHONY: clean
# removes all binaries and any leftover tar packages from Kind build
clean:
	rm -rf bin/ inventory-api.tar inventory-e2e-tests.tar kafka-connect.tar


.PHONY: test
# run all tests
test:
	@echo ""
	@echo "Running tests."
	# TODO: e2e tests are taking too long to be enabled by default. They need to be sped up.
	@$(GO) test ./... -count=1 -race -covermode=atomic -coverprofile=coverage.txt -skip 'TestInventoryAPIGRPC_*|TestInventoryAPIHTTP_*|Test_ACMKafkaConsumer'
	@echo "Overall test coverage:"
	@$(GO) tool cover -func=coverage.txt | grep total: | awk '{print $$3}'

.PHONY: check-e2e-tests
# check result of kind e2e tests
check-e2e-tests:
	./scripts/check-e2e-tests.sh

test-coverage: test
	@$(GO) tool cover -html=coverage.txt -o coverage.html
	@echo "coverage report written to coverage.html"


.PHONY: generate
# generate
generate:
	$(GO) mod tidy
	$(GO) get github.com/google/wire/cmd/wire@latest
	$(GO) generate ./...

.PHONY: all
# generate all
all:
	make api;
	# make config;
	make generate;

.PHONY: lint
# run go linter with the repositories lint config
lint:
	@echo "Linting code."
	@$(DOCKER) run -t --rm -v $(PWD):/app -w /app golangci/golangci-lint golangci-lint run -v

.PHONY: pr-check
# generate pr-check
pr-check:
	make generate;
	make test;
	make lint;
	make local-build;
	#

.PHONY: inventory-up
inventory-up:
	./scripts/start-inventory.sh full-setup 8000 9000

.PHONY: inventory-up-relations-ready
inventory-up-relations-ready:
	./scripts/start-inventory.sh full-setup-relations-ready 8081 9081

.PHONY: inventory-up-w-monitoring
inventory-up-w-monitoring:
	./scripts/start-inventory.sh full-kessel-w-monitoring 8081 9081

.PHONY: inventory-up-split
inventory-up-split:
	./scripts/start-inventory.sh split-setup 8000 9000

.PHONY: inventory-up-split-relations-ready
inventory-up-split-relations-ready:
	./scripts/start-inventory.sh split-setup-relations-ready 8081 9081

.PHONY: inventory-up-sso
inventory-up-sso:
	./scripts/start-inventory-kc.sh full-setup-w-sso 8081 9081

.PHONY: inventory-up-kind
inventory-up-kind:
	./scripts/start-inventory-kind.sh

.PHONY: get-token
get-token:
	./scripts/get-token.sh

.PHONY: inventory-down
inventory-down:
	./scripts/stop-inventory.sh

.PHONY: inventory-down-kind
inventory-down-kind:
	./scripts/stop-inventory-kind.sh

.PHONY: update-local-dashboards
update-local-dashboards:
	./scripts/update-local-dashboards.sh

.PHONY: run
# run api locally
run: local-build
	$(GO) run main.go serve

run-help: local-build
	$(GO) run main.go serve --help

.PHONY: migrate
# run database migrations
migrate: local-build
	./bin/inventory-api migrate --config .inventory-api.yaml

.PHONY: update-schema
# fetch the latest schema from github.com/RedHatInsights/kessel-config
update-schema:
	./scripts/get-schema-yaml.sh > ./deploy/schema.yaml

help:
# show help
	@echo ''
	@echo 'Usage:'
	@echo ' make [target]'
	@echo ''
	@echo 'Targets:'
	@awk '/^[a-zA-Z\-_0-9]+:/ { \
	helpMessage = match(lastLine, /^# (.*)/); \
		if (helpMessage) { \
			helpCommand = substr($$1, 0, index($$1, ":")); \
			helpMessage = substr(lastLine, RSTART + 2, RLENGTH); \
			printf "\033[36m%-22s\033[0m %s\n", helpCommand,helpMessage; \
		} \
	} \
	{ lastLine = $$0 }' $(MAKEFILE_LIST)

.DEFAULT_GOAL := help

### Feature branch feature-RHCLOUD-38543 Helpers

.SILENT: check-kafka-status
check-kafka-status:
	echo "Kafka Cluster Ready: $(shell oc get kafka inventory-kafka -o jsonpath='{.status.conditions[].status}')"
	echo "Kafka Connect Ready: $(shell oc get kc inventory-kafka-connect -o jsonpath='{.status.conditions[].status}')"
	echo "Kafka Connector Ready: $(shell oc get kctr kessel-inventory-source-connector -o jsonpath='{.status.conditions[].status}')"
	echo "Kafka Tuple Topic Ready: $(shell oc get kt outbox.event.kessel.tuples -o jsonpath='{.status.conditions[].status}')"
	echo "Kafka Resources Topic Ready: $(shell oc get kt outbox.event.kessel.resources -o jsonpath='{.status.conditions[].status}')"

.PHONY: create-test-notification
create-test-notification:
	curl -H "Content-Type: application/json" -d @data/testData/v1beta1/notifications-integrations.json localhost:8000/api/inventory/v1beta1/resources/notifications-integrations

.PHONY: update-test-notification
update-test-notification:
	curl -X PUT -H "Content-Type: application/json" -d @data/testData/v1beta1/notifications-integrations.json localhost:8000/api/inventory/v1beta1/resources/notifications-integrations

.PHONY: delete-test-notification
delete-test-notification:
	curl -X DELETE -H "Content-Type: application/json" -d @data/testData/v1beta1/notifications-integration-reporter.json localhost:8000/api/inventory/v1beta1/resources/notifications-integrations

.PHONY: check-tuple-messages
check-tuple-messages:
	oc rsh inventory-kafka-connect-connect-0 bin/kafka-console-consumer.sh --bootstrap-server inventory-kafka-kafka-bootstrap:9092 --topic outbox.event.kessel.tuples --from-beginning --property print.headers=true --property print.key=true

.PHONY: check-resource-messages
check-resource-messages:
	oc rsh inventory-kafka-connect-connect-0 bin/kafka-console-consumer.sh --bootstrap-server inventory-kafka-kafka-bootstrap:9092 --topic outbox.event.kessel.resources --from-beginning --property print.headers=true --property print.key=true

.PHONY: check-tuple
check-tuple:
	MESSAGE='{"filter":{"resource_id":"4321","resource_type":"integration","resource_namespace":"notifications","relation":"t_workspace","subject_filter":{"subject_type":"workspace","subject_namespace":"rbac","subject_id":"1234"}}}' && \
	grpcurl -plaintext -d $${MESSAGE} localhost:9000 kessel.relations.v1beta1.KesselTupleService.ReadTuples

.PHONY: check-token-update
check-token-update:
	psql "postgresql://${DB_USER}:${DB_PASSWORD}@localhost:${LOCAL_DB_PORT}/${DB_NAME}" -x -c "select id,inventory_id,consistency_token,workspace_id,reporter from resources;"

