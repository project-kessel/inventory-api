package authn

import (
	"fmt"

	"github.com/go-kratos/kratos/v2/log"

	"github.com/project-kessel/inventory-api/internal/authn/aggregator"
	"github.com/project-kessel/inventory-api/internal/authn/api"
	"github.com/project-kessel/inventory-api/internal/authn/factory"
)

func New(config CompletedConfig, logger *log.Helper) (api.Authenticator, error) {
	if config.Authenticator == nil {
		return nil, fmt.Errorf("authenticator configuration is required")
	}

	// Create the aggregating authenticator based on strategy type
	var aggregatingAuth aggregator.AggregatingAuthenticator
	switch config.Authenticator.Type {
	case aggregator.FirstMatch:
		firstMatch := aggregator.NewFirstMatch()
		// Set logger for debugging which authenticator allowed
		firstMatch.SetLogger(logger)
		aggregatingAuth = firstMatch
	default:
		return nil, fmt.Errorf("unknown authenticator strategy type: %s", config.Authenticator.Type)
	}

	// Create authenticators from chain configs (only for enabled ones)
	authenticatorsAdded := 0
	for i, chainConfig := range config.Authenticator.ChainConfigs {
		// Skip disabled authenticators
		if !chainConfig.Enabled {
			logger.Infof("Skipping disabled authenticator '%s' at chain index %d", chainConfig.Type, i)
			continue
		}

		var auth api.Authenticator
		var err error

		switch chainConfig.Type {
		case string(factory.TypeOIDC):
			if chainConfig.OIDCConfig == nil {
				return nil, fmt.Errorf("oidc authenticator requires config at chain index %d", i)
			}
			logger.Infof("Loading OIDC info from %s", chainConfig.OIDCConfig.AuthorizationServerURL)
			auth, err = factory.CreateAuthenticator(factory.TypeOIDC, chainConfig.OIDCConfig)
			if err != nil {
				return nil, fmt.Errorf("failed to create oidc authenticator: %w", err)
			}

		case string(factory.TypeGuest):
			logger.Info("Allowing unauthenticated access")
			auth, err = factory.CreateAuthenticator(factory.TypeGuest, nil)
			if err != nil {
				return nil, fmt.Errorf("failed to create guest authenticator: %w", err)
			}

		case string(factory.TypeXRhIdentity):
			logger.Info("Will check for x-rh-identity header")
			auth, err = factory.CreateAuthenticator(factory.TypeXRhIdentity, nil)
			if err != nil {
				return nil, fmt.Errorf("failed to create x-rh-identity authenticator: %w", err)
			}

		default:
			return nil, fmt.Errorf("unknown authenticator type in chain at index %d: %s", i, chainConfig.Type)
		}

		aggregatingAuth.Add(auth)
		authenticatorsAdded++
	}

	// Validate that at least one authenticator was successfully added to the chain
	// This prevents silent failures where all authenticators fail to create or are disabled
	// Note: The config validation in config.go already checks that at least one authenticator is enabled,
	// but this provides an additional safety check in case all enabled authenticators fail to create
	// (which should have been caught above, but this is a defensive check).
	if authenticatorsAdded == 0 {
		return nil, fmt.Errorf("no authenticators were successfully created or enabled in the authentication chain")
	}

	return aggregatingAuth, nil
}
