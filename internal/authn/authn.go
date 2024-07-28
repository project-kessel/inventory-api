package authn

import (
	"github.com/project-kessel/inventory-api/internal/authn/api"
	"github.com/project-kessel/inventory-api/internal/authn/clientcert"
	"github.com/project-kessel/inventory-api/internal/authn/delegator"
	//	"github.com/project-kessel/inventory-api/internal/authn/guest"
	"github.com/project-kessel/inventory-api/internal/authn/oidc"
	"github.com/project-kessel/inventory-api/internal/authn/psk"
)

func New(config CompletedConfig) (api.Authenticator, error) {
	d := delegator.New()

	// client certs authn
	d.Add(clientcert.New())

	// pre shared key authn
	if config.PreSharedKeys != nil {
		a := psk.New(*config.PreSharedKeys)
		d.Add(a)
	}

	// oidc tokens
	if config.Oidc != nil {
		if a, err := oidc.New(*config.Oidc); err == nil {
			d.Add(a)
		} else {
			return nil, err
		}
	}

	// unauthenticated
	// TODO: make it configurable whether we allow unauthenticated access
	// d.Add(guest.New())

	return d, nil
}
