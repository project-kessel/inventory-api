package authn

import (
	"fmt"

	"github.com/go-kratos/kratos/v2/log"

	"github.com/project-kessel/inventory-api/internal/authn/api"
	"github.com/project-kessel/inventory-api/internal/authn/clientcert"
	"github.com/project-kessel/inventory-api/internal/authn/delegator"

	"github.com/project-kessel/inventory-api/internal/authn/guest"
	"github.com/project-kessel/inventory-api/internal/authn/oidc"
	"github.com/project-kessel/inventory-api/internal/authn/psk"
)

func New(config CompletedConfig, logger *log.Helper) (api.Authenticator, error) {
	d := delegator.New()

	// client certs authn
	logger.Info("Will check for client certs")
	d.Add(clientcert.New())

	// pre shared key authn
	if config.PreSharedKeys != nil {
		logger.Infof("Loading pre-shared-keys from %s", config.PreSharedKeys.PreSharedKeyFile)
		a := psk.New(*config.PreSharedKeys)
		d.Add(a)
	}

	// oidc tokens
	if config.Oidc != nil {
		logger.Infof("Loading OIDC info from %s", config.Oidc.AuthorizationServerURL)
		if a, err := oidc.New(*config.Oidc); err == nil {
			d.Add(a)
		} else {
			return nil, fmt.Errorf("failed to load OIDC info: %w", err)
		}
	}

	if config.AllowUnauthenticated {
		logger.Info("Allowing unauthenticated access")
		d.Add(guest.New())
	}

	return d, nil
}
