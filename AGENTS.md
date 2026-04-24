# AI Agent Guidelines for Inventory API

## Docs Index

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

**Tiny types**: Use Go defined types on primitives (`type LockId string`, `type Version uint`, `type ResourceId uuid.UUID`) -- not structs with a single field. Tiny types make interfaces self-documenting, prevent argument transposition, and provide a home for domain-specific methods. Each tiny type gets a `New*` constructor (with invariant validation), a `Serialize()` method (using the generic helpers `SerializeString`, `SerializeUint`, `SerializeUUID`, `SerializeBool` from `common.go`), and a `String()` method where appropriate. A corresponding `Deserialize*` function bypasses validation for reconstruction from trusted sources. See `common.go` for the full pattern and generic helpers.

**Struct value objects**: Use unexported fields, a `New*` constructor, and minimal getters. Each struct type gets its own file for a flat view of the domain (e.g., `relationship.go`, `fencing_check.go`). Tightly coupled types may share a file.

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

### Common Operations
- **Adding new resource types**: Update protobuf definitions, regenerate code, add schema validation
- **Modifying APIs**: Use buf.build for breaking change detection, update both gRPC and HTTP
- **Database changes**: Create migrations with advisory locks, update GORM models with validation
- **Adding integrations**: Follow ClowdApp patterns for configuration, implement health checks
- **Performance optimization**: Profile with pprof, use streaming for large datasets, monitor metrics