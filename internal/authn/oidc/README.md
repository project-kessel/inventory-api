# OAuth2 / OIDC Authentication

The oidc package provides an authenticator for OIDC tokens.  We need to make the claims configurable and
figure out how to map them to `Identity` attributes.

`Principal` mapped to `{domain}:{sub}` for example issuer is `sso.redhat.com` and subject `sub` is `1234` so the principal would be `redhat.com:12324`