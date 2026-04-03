# Security Guidelines for Inventory API

## Authentication Chain Configuration

### First-Match Aggregation Strategy
- Use `first_match` strategy for authentication chain aggregation
- Chain authenticators in order of precedence: `x-rh-identity`, `oidc`, `allow-unauthenticated`
- At least one authenticator must be enabled per protocol (HTTP/gRPC)

```yaml
authn:
  authenticator:
    type: first_match
    chain:
      - type: x-rh-identity
        enable: true
        transport: {http: true, grpc: true}
```

### Bearer Token Security Policy
- OIDC authenticator DENIES invalid tokens (no fallback to guest access)
- Presence of `Authorization: Bearer` header indicates explicit auth attempt
- Invalid JWT verification must return `api.Deny`, not `api.Ignore`
- Use `util.GetBearerToken()` for consistent token extraction

### Claims Context Management
- Store claims in `AuthzContext` as authoritative source via `ensureAuthzContext()`
- Use `validateAuthDecision()` for consistent decision validation
- Preserve raw tokens in context for backward compatibility only
- Decision validation: `Allow` requires non-nil claims, `Deny`/`Ignore` are terminal

## Authorization Integration

### Kessel Relations-API Client
- Use gRPC client with proper bearer token authentication
- Implement token caching with 5-minute expiration via `tokenClient`
- Default to `minimize_latency` consistency for non-existent resources
- Use `at_least_as_fresh` consistency when resource exists (with token)

### Bulk Authorization Patterns
- Prefer `CheckBulk` over individual `Check` calls for performance
- Use consistency tokens to maintain read-after-write semantics
- Implement workspace-scoped authorization with RBAC namespace

### Metrics and Monitoring
- Track authorization success/failure counters per method
- Use OpenTelemetry meter: `inventory_relations_api_success/failure`
- Log authorization decisions at Info level with resource details

## TLS and Transport Security

### Client TLS Configuration
- Support both secure and insecure clients via `InsecureClient` flag
- Use `util.NewClient(insecure bool)` for consistent HTTP client creation
- For insecure mode: set `InsecureSkipVerify: true` in TLS config
- Production deployments should enforce TLS verification

### Database SSL Configuration
- Support PostgreSQL SSL modes: `disable`, `allow`, `prefer`, `require`, `verify-ca`, `verify-full`
- Configure via `sslmode` and `sslrootcert` connection parameters
- Validate SSL mode against allowed values in options validation

### Certificate Management
- Load client certificates from environment variables for E2E testing
- Support CA certificate validation with custom root cert pools
- Handle certificate loading errors gracefully with informative logging

## Input Validation and Sanitization

### Protocol Buffer Validation
- Use `protovalidate.Validator` for all incoming requests
- Apply validation to both unary and streaming gRPC methods
- Return `BadRequest` errors with validation details
- Wrap streaming validation with `requestValidatingWrapper`

### Request Validation Pattern
```go
func Validation(validator protovalidate.Validator) middleware.Middleware {
    return func(handler middleware.Handler) middleware.Handler {
        return func(ctx context.Context, req interface{}) (interface{}, error) {
            if v, ok := req.(proto.Message); ok {
                if err := validator.Validate(v); err != nil {
                    return nil, errors.BadRequest("VALIDATOR", err.Error()).WithCause(err)
                }
            }
            return handler(ctx, req)
        }
    }
}
```

## Security Context and Claims

### Claims Structure and Types
- Support three auth types: `oidc`, `x-rh-identity`, `allow-unauthenticated`
- Extract organization ID from x-rh-identity for multi-tenancy
- Handle User, System, and ServiceAccount identity types from Red Hat platform
- Use typed claims: `SubjectId`, `OrganizationId`, `Issuer`, `ClientID`

### Context Propagation
- Use protocol-aware routing: `protocolRoutingAuthenticator`
- Route by transport kind: HTTP vs gRPC
- Populate `AuthzContext` with protocol and subject information
- Fail closed with `ProtocolUnknown` for unsupported transports

## Token Management

### JWT Token Handling
- Decode JWT claims safely with `DecodeJWTClaims()` after verification
- Check token expiration with `IsJWTTokenExpired()`
- Cache tokens by URL+ClientID composite key
- Use client credentials grant type for service-to-service auth

### Token Context Storage
- Store verified `IDToken` in context via `NewTokenContext()`
- Fallback to raw token string with `NewRawTokenContext()`
- Retrieve tokens with `FromTokenContext()` and `FromRawTokenContext()`
- Maintain token context through middleware chain

## Error Handling and Security Failures

### Authentication Error Patterns
- Return `UNAUTHORIZED` with specific reasons for auth failures
- Use consistent error reasons: "Authentication denied", "No valid authentication found"
- Defensive programming: validate claims are not nil when decision is Allow
- Log authentication failures at Debug level to prevent log flooding

### Authorization Error Handling
- Increment failure metrics on authorization errors
- Log authorization decisions with resource context
- Handle gRPC errors from relations-api gracefully
- Provide meaningful error messages without exposing internal details

## Configuration Security

### Sensitive Data Management
- No plaintext secrets in configuration files
- Use environment variables for credentials and tokens
- Support insecure modes only for development/testing
- Validate configuration options at startup

### OIDC Configuration Security
- Support configurable client ID validation (`skip-client-id-check`)
- Optional audience claim enforcement (`enforce-aud-check`)
- Configurable issuer validation (`skip-issuer-check`)
- Default principal domain to prevent empty principals

## Development and Testing Security

### Test Data and Mocks
- Use example PSKs in test configurations only
- Implement security-aware test utilities in `testutil/`
- Mock authenticators and authorizers for unit tests
- Separate test credentials from production paths

### Environment-Specific Settings
- Support TLS environment variables for E2E testing
- Graceful fallback to insecure mode when certificates unavailable
- Clear separation between development and production security settings