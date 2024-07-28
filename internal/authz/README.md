# Authorization

This package currently has two authorizer implementations: one that allows full access and another that
calls a [kessel](https://github.com/project-kessel) relations-api service.

We're committed to the relations-api as the authorizer interface.  I don't see a need _at this time_ for
higher abstraction or a delegation design like in authn.
