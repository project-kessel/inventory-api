# Authentication

The authn package provides a handful of authn methods:

* Client Certificate
* OAuth2/OIDC tokens
* Pre-Shared Keys
* Guest (non-authenticated)

We need to decide what goes in [api/identity.go](./api/identity.go) and how it gets populated with the
different authn methods.

We might have another method in here for the consoledot rh-identity token.
