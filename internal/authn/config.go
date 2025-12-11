package authn

import (
	"fmt"

	"github.com/project-kessel/inventory-api/internal/authn/aggregator"
	"github.com/project-kessel/inventory-api/internal/authn/oidc"
)

// ChainEntry represents a single authenticator in the chain
type ChainEntry struct {
	Type    string                 `mapstructure:"type"`
	Enabled *bool                  `mapstructure:"enabled"` // nil means enabled=true (default)
	Config  map[string]interface{} `mapstructure:"config"`
}

// AuthenticatorConfig represents the aggregating authenticator configuration
type AuthenticatorConfig struct {
	Type  string       `mapstructure:"type"`
	Chain []ChainEntry `mapstructure:"chain"`
}

// Config represents the authentication configuration
type Config struct {
	Authenticator *AuthenticatorConfig `mapstructure:"authenticator"`
}

func NewConfig(o *Options) *Config {
	cfg := &Config{}

	// Build authenticator config from options
	if o.Authenticator != nil {
		cfg.Authenticator = &AuthenticatorConfig{
			Type:  o.Authenticator.Type,
			Chain: make([]ChainEntry, len(o.Authenticator.Chain)),
		}

		for i, entry := range o.Authenticator.Chain {
			cfg.Authenticator.Chain[i] = ChainEntry(entry)
		}
	}

	return cfg
}

type completedConfig struct {
	Authenticator *AuthenticatorCompletedConfig
}

type CompletedConfig struct {
	*completedConfig
}

// GetOIDCConfig returns the OIDC config from the chain if present, or nil
func (c *CompletedConfig) GetOIDCConfig() *oidc.CompletedConfig {
	if c.Authenticator == nil {
		return nil
	}
	for _, chainConfig := range c.Authenticator.ChainConfigs {
		if chainConfig.Type == "oidc" && chainConfig.OIDCConfig != nil {
			return chainConfig.OIDCConfig
		}
	}
	return nil
}

// AuthenticatorCompletedConfig represents the completed authenticator configuration
type AuthenticatorCompletedConfig struct {
	Type         aggregator.StrategyType
	ChainConfigs []ChainCompletedConfig
}

// ChainCompletedConfig represents a completed chain entry configuration
type ChainCompletedConfig struct {
	Type       string
	Enabled    bool // true if enabled, false if disabled
	OIDCConfig *oidc.CompletedConfig
}

func (c *Config) Complete() (CompletedConfig, []error) {
	var errs []error
	cfg := CompletedConfig{&completedConfig{}}

	if c.Authenticator == nil {
		errs = append(errs, &ConfigError{Message: "authenticator configuration is required"})
		return cfg, errs
	}

	// Validate strategy type
	strategyType := aggregator.StrategyType(c.Authenticator.Type)
	if strategyType != aggregator.FirstMatch {
		errs = append(errs, &ConfigError{
			Message: "invalid authenticator strategy type",
			Value:   c.Authenticator.Type,
		})
		return cfg, errs
	}

	// Process chain entries
	chainConfigs := make([]ChainCompletedConfig, 0, len(c.Authenticator.Chain))
	for i, entry := range c.Authenticator.Chain {
		chainConfig, entryErrs := c.completeChainEntry(entry, i)
		if len(entryErrs) > 0 {
			errs = append(errs, entryErrs...)
			continue
		}
		chainConfigs = append(chainConfigs, chainConfig)
	}

	// Validate that at least one authenticator is enabled
	enabledCount := 0
	for _, chainConfig := range chainConfigs {
		if chainConfig.Enabled {
			enabledCount++
		}
	}
	if enabledCount == 0 {
		errs = append(errs, &ConfigError{
			Message: "at least one authenticator in the chain must be enabled",
		})
		return cfg, errs
	}

	if len(errs) > 0 {
		return cfg, errs
	}

	cfg.Authenticator = &AuthenticatorCompletedConfig{
		Type:         strategyType,
		ChainConfigs: chainConfigs,
	}

	return cfg, nil
}

func (c *Config) completeChainEntry(entry ChainEntry, index int) (ChainCompletedConfig, []error) {
	var errs []error
	// Default to enabled=true if not specified
	enabled := true
	if entry.Enabled != nil {
		enabled = *entry.Enabled
	}
	chainConfig := ChainCompletedConfig{
		Type:    entry.Type,
		Enabled: enabled,
	}

	switch entry.Type {
	case "guest", "x-rh-identity":
		// No config needed
		return chainConfig, nil

	case "oidc":
		if entry.Config == nil {
			errs = append(errs, &ConfigError{
				Message: fmt.Sprintf("oidc authenticator requires config at chain index %d", index),
				Type:    entry.Type,
			})
			return chainConfig, errs
		}
		oidcOpts := oidc.NewOptions()
		if clientID, ok := entry.Config["client-id"].(string); ok {
			oidcOpts.ClientId = clientID
		}
		if authServerURL, ok := entry.Config["authn-server-url"].(string); ok {
			oidcOpts.AuthorizationServerURL = authServerURL
		}
		if insecure, ok := entry.Config["insecure-client"].(bool); ok {
			oidcOpts.InsecureClient = insecure
		}
		if skipClientIDCheck, ok := entry.Config["skip-client-id-check"].(bool); ok {
			oidcOpts.SkipClientIDCheck = skipClientIDCheck
		}
		if enforceAudCheck, ok := entry.Config["enforce-aud-check"].(bool); ok {
			oidcOpts.EnforceAudCheck = enforceAudCheck
		}
		if skipIssuerCheck, ok := entry.Config["skip-issuer-check"].(bool); ok {
			oidcOpts.SkipIssuerCheck = skipIssuerCheck
		}
		if principalUserDomain, ok := entry.Config["principal-user-domain"].(string); ok {
			oidcOpts.PrincipalUserDomain = principalUserDomain
		}
		oidcConfig := oidc.NewConfig(oidcOpts)
		completedOIDC, err := oidcConfig.Complete()
		if err != nil {
			errs = append(errs, &ConfigError{
				Message: fmt.Sprintf("failed to complete oidc config at chain index %d", index),
				Type:    entry.Type,
				Err:     err,
			})
			return chainConfig, errs
		}
		chainConfig.OIDCConfig = &completedOIDC

	default:
		errs = append(errs, &ConfigError{
			Message: fmt.Sprintf("unknown authenticator type at chain index %d", index),
			Type:    entry.Type,
		})
		return chainConfig, errs
	}

	return chainConfig, nil
}

// ConfigError represents a configuration error
type ConfigError struct {
	Message string
	Type    string
	Value   string
	Err     error
}

func (e *ConfigError) Error() string {
	msg := e.Message
	if e.Type != "" {
		msg += " (type: " + e.Type + ")"
	}
	if e.Value != "" {
		msg += " (value: " + e.Value + ")"
	}
	if e.Err != nil {
		msg += ": " + e.Err.Error()
	}
	return msg
}

func (e *ConfigError) Unwrap() error {
	return e.Err
}
