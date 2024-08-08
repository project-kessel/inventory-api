GOHOSTOS:=$(shell go env GOHOSTOS)
GOPATH:=$(shell go env GOPATH)
DOCKER:=$(shell command -v podman || command -v docker)

API_PROTO_FILES:=$(shell find api -name *.proto)

TITLE:="Kessel Asset Inventory API"
VERSION:=$(shell git describe --tags --always)
INVENTORY_SCHEMA_VERSION=0.11.0

.PHONY: init
# init env
init:
	go install github.com/go-kratos/kratos/cmd/kratos/v2@latest
	go install github.com/google/wire/cmd/wire@latest
	go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	go get google.golang.org/grpc/cmd/protoc-gen-go-grpc
	go install google.golang.org/grpc/cmd/protoc-gen-go-grpc
	go install github.com/go-kratos/kratos/cmd/protoc-gen-go-http/v2@latest
	go install github.com/google/gnostic/cmd/protoc-gen-openapi@latest
	go get github.com/envoyproxy/protoc-gen-validate
	go install github.com/envoyproxy/protoc-gen-validate

.PHONY: api
# generate api proto
api:
	@echo "Generating api protos"
	@$(DOCKER) build -t custom-protoc ./api
	@$(DOCKER) run -t --rm -v $(PWD)/api:/api -v $(PWD)/openapi.yaml:/openapi.yaml -v $(PWD)/third_party:/third_party \
	-w=/api/ custom-protoc sh -c "buf generate && \
		buf lint && \
		buf breaking --against 'buf.build/project-kessel/inventory-api' "

.PHONY: api_breaking
# generate api proto
api_breaking:
	@echo "Generating api protos, allowing breaking changes"
	@$(DOCKER) build -t custom-protoc ./api
	@$(DOCKER) run -t --rm -v $(PWD)/api:/api -v $(PWD)/openapi.yaml:/openapi.yaml -v $(PWD)/third_party:/third_party \
	-w=/api/ custom-protoc sh -c "buf generate && \
		buf lint"

# .PHONY: api
# # generate api proto
# api:
# 	@echo "Generating api protos"
# 	@$(DOCKER) build -t custom-protoc ./api
# 	@$(DOCKER) run -t --rm -v $(PWD)/api:/api:rw -v $(PWD)/openapi.yaml:/openapi.yaml:rw -v $(PWD)/third_party:/third_party:rw \
# 	-w=/api/ custom-protoc sh -c "buf generate && buf lint"

.PHONY: build
# build
build:
	mkdir -p bin/ && go build -ldflags "-X cmd.Version=$(VERSION)" -o ./bin/ ./...

.PHONY: test
# run all tests
test:
	@echo ""
	@echo "Running tests."
	go test ./... -count=1

.PHONY: generate
# generate
generate:
	go mod tidy
	go get github.com/google/wire/cmd/wire@latest
	go generate ./...

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
	make build;
	#

.PHONY: inventory-up
inventory-up:
	./scripts/start-inventory.sh

.PHONY: inventory-down
inventory-down:
	./scripts/stop-inventory.sh

.PHONY: run
# run api locally
run: build
	go run main.go serve --config .inventory-api.yaml

.PHONY: migrate
# run database migrations
migrate: build
	./bin/inventory-api migrate --config .inventory-api.yaml


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
