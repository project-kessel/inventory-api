# Presentation Layer Guidelines (Middleware)

This file covers middleware under `internal/middleware/`. For service implementations, see `internal/service/GUIDELINES.md`. For server setup, see `internal/server/GUIDELINES.md`. For cross-cutting concerns, see `internal/GUIDELINES.md`.

## Error Mapping

### gRPC Status Code Mapping
`middleware.MapError()` provides consistent error-to-status mapping:
- Validation errors → `InvalidArgument`
- Authorization errors → `Unauthenticated` or `PermissionDenied`
- Domain errors → Appropriate business codes (`NotFound`, `AlreadyExists`, `FailedPrecondition`)
- Database errors → `Internal`
- Context errors → `Canceled` or `DeadlineExceeded`

### Preserve Existing Status Codes
`MapError` passes through errors that already have gRPC status codes (except `Unknown`)

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

## Authentication Middleware

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

### Context Propagation
- Use protocol-aware routing: `protocolRoutingAuthenticator`
- Route by transport kind: HTTP vs gRPC
- Populate `AuthzContext` with protocol and subject information
- Fail closed with `ProtocolUnknown` for unsupported transports
