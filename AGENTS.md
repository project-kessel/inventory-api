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
5. **Test completeness** - Write tests for all new functionality following established patterns

### Common Operations
- **Adding new resource types**: Update protobuf definitions, regenerate code, add schema validation
- **Modifying APIs**: Use buf.build for breaking change detection, update both gRPC and HTTP
- **Database changes**: Create migrations with advisory locks, update GORM models with validation
- **Adding integrations**: Follow ClowdApp patterns for configuration, implement health checks
- **Performance optimization**: Profile with pprof, use streaming for large datasets, monitor metrics