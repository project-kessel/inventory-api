# AI Agent Guidelines for Inventory API

## Docs Index

**All guidelines in this file and the linked docs are mandatory, not recommendations.** Agents MUST read the relevant guideline doc before making changes in that area and MUST follow every rule. Failure to read is not an excuse for non-compliance.

The following domain-specific guidelines provide detailed, repo-specific conventions for AI agents working in this codebase:

- [Security Guidelines](docs/security-guidelines.md) - Authentication, authorization, TLS, input validation, and security patterns
- [Performance Guidelines](docs/performance-guidelines.md) - Database connections, concurrency, streaming, context management, and optimization
- [Error Handling Guidelines](docs/error-handling-guidelines.md) - Error types, mapping, consumer patterns, retries, and observability
- [API Contracts Guidelines](docs/api-contracts-guidelines.md) - Protobuf definitions, versioning, validation, consistency models, and testing
- [Database Guidelines](docs/database-guidelines.md) - GORM models, transactions, migrations, outbox patterns, and query conventions
- [Testing Guidelines](docs/testing-guidelines.md) - Test organization, infrastructure, fixtures, authorization testing, and coverage
- [Integration Guidelines](docs/integration-guidelines.md) - Kafka consumers, Relations API, health checks, observability, and configuration

## AI Guidance and Repo Conventions

### Repository Context
This is the Kessel Inventory API, a Go-based microservice that provides resource inventory management with:
- **gRPC and HTTP APIs** using Kratos framework with protobuf definitions
- **PostgreSQL/SQLite database** with GORM ORM and serializable transactions
- **Kafka consumer** for event processing with Debezium CDC integration
- **Relations API integration** for authorization via Kessel relations service
- **Multi-protocol authentication** supporting OIDC, X-RH-Identity, and development modes
- **Comprehensive observability** with OpenTelemetry metrics and structured logging

### Key Architectural Patterns
- **Clean Architecture**: Business logic in `internal/biz`, data layer in `internal/data`, service layer in `internal/service`
- **Domain-Driven Design**: Aggregate roots in `model` package with repository patterns
- **Event Sourcing**: Outbox pattern with dual WAL/table modes for reliable event publishing
- **CQRS-like separation**: Read-after-write consistency with Relations API integration
- **Hexagonal ports**: Abstractions for external dependencies (database, auth, messaging)

### Domain Model Conventions (`internal/biz/model`)
Prefer **value objects** (private fields, constructors, getters) over plain data containers (exported fields).

**Tiny types** (e.g., `LockId`, `Version`, `ResourceId`, `ResourceType`, `ReporterType`) make interfaces self-documenting, prevent argument transposition, and provide a home for domain-specific methods like validation and normalization. Always create tiny type values through their `New*` constructor -- never via direct type conversion (e.g., `ReporterType("HBI")`). Direct conversion bypasses validation and normalization (such as lowercasing). Use `NewReporterType("HBI")` instead, which returns the validated, normalized value.

Each tiny type gets:
- A `New*` constructor that validates invariants and normalizes the value
- A `Serialize()` method (using the generic helpers `SerializeString`, `SerializeUint`, `SerializeUUID`, `SerializeBool` from `common.go`)
- A `String()` method where appropriate
- A corresponding `Deserialize*` function that reconstructs the domain type from stored/wire representations (e.g., database rows, protobuf fields) — analogous to how Jackson deserializes JSON into Java objects. The stored value is assumed to be already validated and normalized.

See `common.go` for the full pattern and generic helpers.

**Struct value objects**: Use unexported fields, a `New*` constructor, and minimal getters. Each struct type gets its own file for a flat view of the domain (e.g., `relationship.go`, `fencing_check.go`). Tightly coupled types may share a file.

**Domain services** (e.g., `SchemaService`): Stateless domain services that orchestrate logic over injected repositories can be constructed inside a usecase's `New` function from already-injected dependencies. There is no need to inject them pre-built -- they have no lifecycle, configuration, or external connections of their own. If a domain service ever gains its own dependencies (caching, external API calls, etc.), promote it to a directly injected parameter.

- **Use meaningful variable names** -- `resourceType`, `reporterType`, `reporterInstanceId`, not cryptic abbreviations like `rt`, `rpt`, `ri`.
- **All domain types must use unexported fields** initialized through constructor functions (`New*`).
- **Constructors must validate invariants** (non-empty IDs, required fields, etc.) and return `(T, error)` when validation can fail. Do not silently discard constructor errors.
- **Add getters only when they are actually used** by callers outside the package. Do not add speculative getters.
- **Value objects are immutable** -- no setter methods. To change a value, construct a new instance.
- **`Deserialize*` functions** bypass validation and are reserved for reconstructing values from trusted storage or wire formats (e.g., database rows, protobuf fields).
- **Protobuf and gRPC types must not appear** in model types, repository interfaces, or usecase-layer code. All proto-to-model conversion belongs exclusively in the data layer (`internal/data`).
- **Streaming results** use the generic `ResultStream[T]` interface (wrapping `Recv() (T, error)`) rather than exposing `grpc.ServerStreamingClient`.

### Development Workflow
- **Protocol-first development**: All APIs defined in protobuf with buf.build toolchain
- **Schema-driven validation**: JSON schemas for resource types with tarball deployment
- **Test-driven development**: Comprehensive unit, integration, and E2E test coverage
- **Security-first approach**: All endpoints require authentication with configurable authorization
- **Performance-conscious**: Serializable isolation, connection pooling, streaming for large datasets

### Agent Responsibilities
When working in this codebase, AI agents should:
1. **Follow domain guidelines** - Always consult the relevant domain guideline before implementing changes
2. **Maintain consistency** - Use established patterns for error handling, validation, and testing
3. **Security awareness** - Never bypass authentication/authorization; follow security guidelines
4. **Performance consideration** - Use appropriate transaction patterns and avoid blocking operations
5. **Test-driven development** - Prefer writing tests first (TDD). When adding features or fixing bugs, start with a failing test, then implement the code to pass it. Follow the no-mocks philosophy in [Testing Guidelines](docs/testing-guidelines.md).
6. **End-to-end type changes** - When replacing a type (e.g., `ReporterResourceKey` -> `ResourceReference`), propagate the change through the entire call chain. Do not insert local adapter/shim calls at the boundary (e.g., `ResourceReferenceFromKey(key)` at the call site). Instead, update the method signature, the callers, and the callers' callers until the new type flows naturally from entry point to implementation. Lossy back-and-forth conversions are a bug.
7. **Do not remove comments** unless they are clearly wrong or obsolete. Preserve existing documentation.

### Docker Compose / Local Development
When changing CLI flags, config keys, default values, authentication/authorization modes, ports, or service dependencies, check whether the local development setup still reflects those changes. The following files must stay in sync:

- **Compose config files** (`development/configs/base.yaml`, `authn-sso.yaml`, `authn-rh-identity.yaml`, `local-w-relations.yaml`) - These are the YAML configs mounted into the inventory-api container. If a config key is added, renamed, or its default changes, update all affected config files.
- **Compose files** (`development/docker-compose.yaml`, `development/full-kessel/docker-compose.yaml`) - Service definitions, environment variables, ports, and healthchecks. The inventory-api service uses Viper env var overrides (`INVENTORY_API_*`) for simple config differences between targets; complex nested config (authn chains, OIDC) requires a separate config file.
- **Makefile targets** - The `inventory-up-*` and `kessel-up` targets in the Makefile pass config names, ports, and service lists to the startup scripts. If a new service or config variant is added, add or update the corresponding target.
- **Documentation** (`docs/dev-guides/docker-compose-options.md`, `README.md`) - The docker-compose options guide documents all make targets, ports, and usage instructions. Keep it current when targets or behavior change.

**Viper env var limitation**: The env key replacer in `cmd/root.go` only maps `.` to `_`, not `-` to `_`. Config keys containing hyphens (e.g., `consumer.bootstrap-servers`) cannot be overridden via `INVENTORY_API_*` environment variables. Only dot-separated keys work as env overrides.

### Common Operations
- **Adding new resource types**: Update protobuf definitions, regenerate code, add schema validation
- **Modifying APIs**: Use buf.build for breaking change detection, update both gRPC and HTTP
- **Database changes**: Create migrations with advisory locks, update GORM models with validation
- **Adding integrations**: Follow ClowdApp patterns for configuration, implement health checks
- **Performance optimization**: Profile with pprof, use streaming for large datasets, monitor metrics