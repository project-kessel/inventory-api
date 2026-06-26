# Infrastructure Layer Guidelines (Authentication)

This file covers authentication providers under `internal/authn/`. For presentation-layer authentication concerns (middleware, context propagation), see `internal/service/GUIDELINES.md` and `internal/middleware/GUIDELINES.md`. For cross-cutting concerns, see `internal/GUIDELINES.md`.

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

## OIDC Configuration Security
- Support configurable client ID validation (`skip-client-id-check`)
- Optional audience claim enforcement (`enforce-aud-check`)
- Configurable issuer validation (`skip-issuer-check`)
- Default principal domain to prevent empty principals
- Default security posture: client ID check ON, issuer check ON, audience check ON.
- Any skip/disable option is allowed only in non-production environments and must be explicitly documented in deployment config.

## Authentication Error Patterns
- Return `UNAUTHORIZED` with specific reasons for auth failures
- Use consistent error reasons: "Authentication denied", "No valid authentication found"
- Defensive programming: validate claims are not nil when decision is Allow
- Log authentication failures at Debug level to prevent log flooding

## Multi-Strategy Authentication
- **Authenticator chains** support multiple auth methods via factory pattern
- **OIDC integration** with configurable auth server URL and client credentials
- **Unauthenticated fallback** for development scenarios only
  - ⚠️ **Security Warning**: `allow-unauthenticated` bypasses all authentication
  - Should never be used in production environments
  - Requires explicit configuration and awareness of security implications
- **Bearer token extraction** from request context for downstream calls
