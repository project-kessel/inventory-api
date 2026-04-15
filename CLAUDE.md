# Kessel Inventory API - Claude Code Configuration

## Project Context

This is the Kessel Inventory API, a Go-based microservice for resource inventory management using:
- **gRPC and HTTP APIs** with protobuf definitions and Kratos framework
- **PostgreSQL/SQLite** with GORM ORM and serializable transactions  
- **Kafka integration** for event processing via Debezium CDC
- **Relations API** integration for authorization
- **Multi-protocol authentication** (OIDC, X-RH-Identity, development modes)

## Build and Test Commands

```bash
# Build the project
make local-build

# Run tests
make test

# Run with coverage
make test-coverage

# Setup local database
make db/setup

# Run migrations
make migrate

# Start development server
make run

# Generate API code from protobuf
make api

# Build container images
make docker-build-push
```

## Development Guidelines

- Follow the domain-specific guidelines in `docs/` for consistent patterns
- Use `buf.build` toolchain for protobuf development and breaking change detection
- All database operations should use the transaction manager with serializable isolation
- Write comprehensive tests following the established patterns in `testframework_test.go`
- Security-first approach: never bypass authentication/authorization
- Performance-conscious: use streaming for large datasets, avoid blocking operations

## Architecture Notes

- **Clean Architecture**: `internal/biz` (business), `internal/data` (persistence), `internal/service` (API)
- **Event Sourcing**: Outbox pattern with WAL/table modes for reliable event publishing
- **CQRS**: Read-after-write consistency with Relations API integration
- **Hexagonal**: Port abstractions for external dependencies

@AGENTS.md