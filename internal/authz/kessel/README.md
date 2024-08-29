# Authorizer Using the Kessel Relations API

Configure the `impl` as `Kessel`

```yaml
authz:
  impl: kessel
  kessel:
    insecure-client: true
    url: localhost:9000
    enable-oidc-auth: true
    sa-client-id: "svc-test"
    sa-client-secret: "<secret>"
    sso-token-endpoint: "http://localhost:8084/realms/redhat-external/protocol/openid-connect/token"
```