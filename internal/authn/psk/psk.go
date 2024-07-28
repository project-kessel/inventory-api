// Package psk provides an authenticator based on pre-shared keys
package psk

import (
	"net/http"

	"github.com/project-kessel/inventory-api/internal/authn/api"
	"github.com/project-kessel/inventory-api/internal/authn/util"
)

type IdentityMap map[string]api.Identity

type PreSharedKeyAuthenticator struct {
	Store IdentityMap
}

func New(config CompletedConfig) *PreSharedKeyAuthenticator {
	return &PreSharedKeyAuthenticator{Store: config.Keys}
}

func (a *PreSharedKeyAuthenticator) Lookup(key string) *api.Identity {
	if len(key) > 0 {
		if identity, found := a.Store[key]; found {
			return &identity
		}
	}
	return nil
}

func (a *PreSharedKeyAuthenticator) Authenticate(r *http.Request) (*api.Identity, api.Decision) {
	token := util.GetBearerToken(r)
	identity := a.Lookup(token)
	if identity != nil {
		return identity, api.Allow
	}
	return nil, api.Ignore
}
