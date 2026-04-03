# Inventory API Integration Guidelines

## Kafka Consumer Integration

### Message Processing Pattern
- Use **structured operation handlers** with `operationConfig` for consistent create/update/delete operations
- Process messages using **Debezium CDC events** with required headers: `operation`, `txid`
- Implement **fencing tokens** via Relations API for consumer lock management during rebalancing
- Follow **batch offset commits** using `commitModulo` configuration for performance

### Consumer Configuration
- Bootstrap servers from environment or ClowdApp config via `InjectClowdAppConfig()`
- Use **SASL authentication** when `AuthConfig.Enabled` is true
- Configure **retry policies** with exponential backoff: `BackoffFactor`, `MaxBackoffSeconds`
- Set **statistics interval** for metrics collection (default: consumer stats every interval)

### Error Handling
- **Graceful shutdown** coordinates offset commits during rebalance via `offsetMutex`
- **Retry operations** using `Retry()` method with configurable max attempts
- **Metrics collection** for failures: `MsgProcessFailures`, `ConsumerErrors`, `KafkaErrorEvents`

## Relations API Integration

### Client Authentication
- Use **OIDC token client** when `EnableOIDCAuth` is true
- Apply **Bearer token** auth via `WithBearerToken()` or `WithInsecureBearerToken()`
  - ⚠️ **Security Warning**: `WithInsecureBearerToken()` bypasses TLS verification and should never be used in production
- Handle **authentication failures** with proper error metrics

### Tuple Operations
- **Convert internal models** to protobuf using `convertTuplesToRelationships()`
- Use **upsert semantics** for CreateTuples to handle existing relationships
- Implement **consistency tokens** for read-after-write guarantees via `UpdateConsistencyToken()`
- Apply **fencing checks** for all tuple operations to prevent race conditions

### Bulk Operations
- Use **CheckBulk/CheckForUpdateBulk** for authorization checks on multiple resources
- Default to **minimize_latency** consistency when no token available
- Use **at_least_as_fresh** consistency when token is present from previous operations

## Health Check Implementation

### Service Health Patterns
- **Livez endpoint**: Simple OK response, configurable logging via `log.livez`
- **Readyz endpoint**: Check backend availability (database, relations-api)
- **Relations health**: Call Relations API `/readyz` endpoint with proper auth
- Use **helper logging** to avoid spam: log disable messages only once

### Database Health
- Check **PostgreSQL connectivity** via GORM connection
- Validate **migration state** if applicable
- Return **structured responses** with status codes (200 for OK)

## Observability Integration

### OpenTelemetry Setup
- **Meter provider** with Prometheus exporter for metrics
- **Service name**: Always use "inventory-api" for consistent identification
- **Default histogram views** for HTTP/gRPC server duration metrics
- **Resource attributes** using semantic conventions

### Metrics Collection
- **Consumer stats** with prefixed names: `consumer_stats_*`, `consumer_*`
- **Relations API calls** tracked with success/failure counters per method
- **Custom metrics** using `kessel_inventory_*` prefix for application-specific metrics
- **Label consistency**: Use structured attributes for partition, topic, operation

### Metrics Patterns
```go
// Use helper function for consistent labeling
metricscollector.Incr(counter, operation, attribute.String("extra", value))
```

## Configuration Management

### ClowdApp Integration
- **Automatic discovery** of dependent services via `InjectClowdAppConfig()`
- **Database configuration** from ClowdApp database section
- **Kafka brokers** extracted from ClowdApp kafka configuration
- **RDS CA certificate** handling for TLS connections

### Environment-Based Config
- **Structured options pattern**: Each component has `*Options` and `CompletedConfig`
- **Validation on completion**: Return errors array from `Complete()` methods
- **Debug logging**: Use `LogConfigurationInfo()` to output non-sensitive config details

## Schema Validation

### JSON Schema Integration
- **gojsonschema library** for validation with structured error reporting
- **ValidationSchema interface** for consistent validation across different data types
- **Error aggregation**: Combine multiple validation errors into single error message

### Resource Schema Management
- **Schema repository pattern** with in-memory caching
- **Resource type definitions** loaded from tarball during deployment
- **Representation validation** against reporter-specific schemas

## Testing Patterns

### E2E Testing Setup
- **Kafka consumer tests** with schema validation against CloudEvents specification
- **Environment configuration** via env vars with sensible defaults
- **Topic creation** using admin client before test execution
- **Message filtering**: Skip delete tombstones for event schema validation

### Integration Test Patterns
```go
// Standard test timeout for async operations
ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
defer cancel()
```

## Service Discovery

### ClowdApp Dependencies
- **Optional dependencies** declared in deployment template for Relations API
- **Hostname/port resolution** via ClowdApp endpoint configuration
- **Automatic URL construction** for dependent service connections

### Configuration Injection
```go
// Standard pattern for ClowdApp integration
func (o *OptionsConfig) InjectClowdAppConfig(appconfig *clowder.AppConfig) error
```

## Outbox Pattern Implementation

### Dual Publishing Strategy
- **WAL logical decoding** mode for high performance (`storage.OutboxModeWAL`)
- **Table-based outbox** as fallback for compatibility
- **Message format consistency** between both approaches using `walOutboxMessage`

### Transaction Coordination
- **Outbox events** created within same transaction as domain changes
- **UUID generation** using V7 for time-ordered IDs
- **Payload preservation** using `internal.JsonObject` for flexible content

## Authentication Integration

### Multi-Strategy Authentication
- **Authenticator chains** support multiple auth methods via factory pattern
- **OIDC integration** with configurable auth server URL and client credentials
- **Unauthenticated fallback** for development scenarios only
  - ⚠️ **Security Warning**: `allow-unauthenticated` bypasses all authentication
  - Should never be used in production environments
  - Requires explicit configuration and awareness of security implications
- **Bearer token extraction** from request context for downstream calls

### Authorization Patterns
- **Kessel authz** for production with Relations API integration
- **Allow-all authz** for testing and development scenarios only
  - ⚠️ **Security Warning**: `allow-all` bypasses all authorization checks
  - Should never be used in production environments  
  - Requires explicit configuration and awareness of security implications
- **Subject reference construction** from authenticated identity context