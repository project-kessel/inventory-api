# Authentication

The authn package implements the **Application defined Authenticator (single)** pattern, where a single aggregating authenticator encapsulates the authentication strategy and manages a chain of underlying authenticators.

## Architecture Pattern

This implementation follows the recommended pattern where:

- **Single Authenticator Interface**: The application interacts with one unified authenticator interface
- **Encapsulated Aggregation**: The aggregation strategy is defined in the authenticator's configuration, not in application code
- **Chain of Authenticators**: Multiple authenticators are chained together, with the aggregation strategy determining how their results are combined

### Benefits

- **Simple Interaction**: Every authentication interaction is straightforward - provide request context, get user identity
- **Centralized Strategy**: New aggregation strategies are implemented in one place (e.g., `first_match`)
- **Easy Testing**: Authentication logic is easy to test - every interaction is simple, and aggregating authenticators can be unit tested in isolation
- **Flexible Configuration**: The aggregation strategy is configurable, allowing different strategies without code changes

## Authentication Methods

The package supports the following authentication methods:

- **OAuth2/OIDC** (`oidc`) - Authenticates using OAuth2/OIDC JWT tokens from the `Authorization: Bearer` header
- **x-rh-identity** (`x-rh-identity`) - Authenticates using the `x-rh-identity` header from Red Hat ConsoleDot/Cloud Platform
- **Allow unauthenticated** (`allow-unauthenticated`) - Allows unauthenticated access (uses User-Agent as principal). `guest` is accepted as a legacy alias.

## Aggregation Strategy

The authentication system uses an aggregating authenticator pattern where multiple authenticators can be chained together using the `first_match` strategy:

- **first_match** - Allows the request if any authenticator returns Allow. Only denies if all authenticators return Deny. This is useful when a request might have multiple authentication methods (e.g., both `x-rh-identity` header and `Authorization: Bearer` token), and you want to accept whichever is valid.

## Configuration

Authentication is configured using a nested structure that specifies the aggregation strategy and the chain of authenticators:

```yaml
authn:
  authenticator:
    type: first_match
    chain:
      - type: x-rh-identity  # Check x-rh-identity header first
        enable_http: false
        enable_grpc: false
      - type: oidc            # Then check Authorization: Bearer token
        enable_http: false
        enable_grpc: false
        config:
          authn-server-url: https://auth.example.com
          client-id: my-client-id
          principal-user-domain: example.com
      - type: allow-unauthenticated           # Finally allow unauthenticated access
        enable_http: true
        enable_grpc: true
```

### Example: Disabling Authenticators

You can disable specific authenticators without removing them from the configuration:

```yaml
authn:
  authenticator:
    type: first_match
    chain:
      - type: x-rh-identity
        enable_http: true
        enable_grpc: true    # Enabled - will check x-rh-identity header
      - type: oidc
        enable_http: false
        enable_grpc: false   # Disabled - will be skipped
        config:
          authn-server-url: https://auth.example.com
          client-id: my-client-id
      - type: allow-unauthenticated
        enable_http: true
        enable_grpc: true    # Enabled - fallback to unauthenticated access
```

**Note**: At least one authenticator in the chain must be enabled for HTTP and gRPC. If either protocol has all authenticators disabled, configuration validation will fail.

### Configuration Fields

**authenticator.type**: The aggregation strategy (currently only `first_match` is supported)

**authenticator.chain**: An ordered list of authenticators to try. Each entry has:

- `type`: One of `oidc`, `x-rh-identity`, or `allow-unauthenticated` (`guest` is accepted as a legacy alias)
- `enable_http`: Boolean to enable/disable this authenticator for HTTP (optional)
- `enable_grpc`: Boolean to enable/disable this authenticator for gRPC (optional)
  - If **both** `enable_http` and `enable_grpc` are **omitted**, the authenticator is **enabled for both protocols by default**.
  - If only one is set, the other is enabled by default.
- `config`: Optional configuration map (required for `oidc`, not needed for `x-rh-identity` or `allow-unauthenticated`)

### OIDC Configuration

When using the `oidc` authenticator, the following config fields are available:

- `authn-server-url`: URL of the OIDC authorization server (required)
- `client-id`: OAuth2 client ID (required)
- `principal-user-domain`: Domain to qualify principal IDs (optional, defaults to "localhost")
- `insecure-client`: Skip TLS certificate validation (default: false)
- `skip-client-id-check`: Skip client ID validation (default: false)
- `enforce-aud-check`: Enforce audience claim check (default: false)
- `skip-issuer-check`: Skip issuer validation (default: false)

## Identity Structure

The `Identity` struct includes:

- `Principal`: The authenticated principal identifier
- `Groups`: Group memberships
- `AuthType`: The authentication method used (`oidc`, `x-rh-identity`, or `allow-unauthenticated`)
- `IsGuest`: Whether this is a guest identity
- `IsReporter`: Whether this is a reporter identity (for client cert auth)

The `AuthType` field is used by authorization middleware to make access control decisions.
