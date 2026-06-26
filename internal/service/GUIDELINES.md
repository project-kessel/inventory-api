# Presentation Layer Guidelines (Service Implementations)

This file covers the gRPC/HTTP service implementations under `internal/service/`. For server setup (TLS, pprof), see `internal/server/GUIDELINES.md`. For middleware (validation, error mapping), see `internal/middleware/GUIDELINES.md`. For cross-cutting concerns, see `internal/GUIDELINES.md`.

## HTTP/gRPC Mapping Conventions
- Most endpoints use POST, health checks use GET
- URL pattern: `/api/kessel/{version}/{operation}`
- Bulk operations append "bulk" to endpoint name
- Self operations append "self" to endpoint name
- Resource CRUD uses `/resources` path with appropriate HTTP methods

## Error Mapping Strategy

### gRPC Status Code Mapping
Use `middleware.MapError()` for consistent error-to-status mapping:
- Validation errors → `InvalidArgument`
- Authorization errors → `Unauthenticated` or `PermissionDenied`
- Domain errors → Appropriate business codes (`NotFound`, `AlreadyExists`, `FailedPrecondition`)
- Database errors → `Internal`
- Context errors → `Canceled` or `DeadlineExceeded`

### Preserve Existing Status Codes
`MapError` passes through errors that already have gRPC status codes (except `Unknown`)

## Streaming Implementation

### gRPC Streaming
```go
func (s *InventoryService) StreamedListObjects(req *pb.StreamedListObjectsRequest, stream pb.KesselInventoryService_StreamedListObjectsServer) error {
    for {
        resp, err := clientStream.Recv()
        if err == io.EOF { return nil }
        if err := stream.Send(ToLookupResourceResponse(resp)); err != nil { return err }
    }
}
```

**Rules:**
- Use direct stream forwarding without buffering for large datasets
- Check for `io.EOF` to detect normal stream completion
- Always validate stream context cancellation
- Convert between internal and external protobuf types at streaming boundaries
- Use stream context for gRPC streaming operations via `stream.Context()`

### Streaming Response Guidelines
- Consider server streaming for list operations returning large result sets
- Include pagination in streaming requests
- Maintain consistency guarantees across stream chunks
- Handle client disconnection gracefully

## Authentication (Presentation Concerns)

### First-Match Aggregation Strategy
- Use `first_match` strategy for authentication chain aggregation
- Chain authenticators in order of precedence: `x-rh-identity`, `oidc`, `allow-unauthenticated`
- At least one authenticator must be enabled per protocol (HTTP/gRPC)

### Bearer Token Security Policy
- OIDC authenticator DENIES invalid tokens (no fallback to guest access)
- Presence of `Authorization: Bearer` header indicates explicit auth attempt
- Invalid JWT verification must return `api.Deny`, not `api.Ignore`
- Use `util.GetBearerToken()` for consistent token extraction

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

## Authentication Error Patterns
- Return `UNAUTHORIZED` with specific reasons for auth failures
- Use consistent error reasons: "Authentication denied", "No valid authentication found"
- Defensive programming: validate claims are not nil when decision is Allow
- Log authentication failures at Debug level to prevent log flooding

## Input Validation

### Protocol Buffer Validation
- Use `protovalidate.Validator` for all incoming requests
- Apply validation to both unary and streaming gRPC methods
- Return `BadRequest` errors with validation details
- Wrap streaming validation with `requestValidatingWrapper`

See `internal/middleware/GUIDELINES.md` for the validation middleware implementation.

## Testing

### Dual Protocol Service Testing
- **Framework**: `testframework_test.go` enables testing both gRPC and HTTP
- **Entry point**: Use `runServerTest(t, factory)` for service layer tests
- **Request building**: `withBody(req, Check, httpEndpoint("POST /path"))`
- **Assertions**: `Assert(t, res, requireError(codes.InvalidArgument))`
- **Response extraction**: `resp := Extract(t, res, expectSuccess(func() *pb.Response{}))`
