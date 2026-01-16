package authn

import (
	"fmt"

	"github.com/project-kessel/inventory-api/internal/authn/aggregator"
	"github.com/project-kessel/inventory-api/internal/authn/oidc"
)

// OIDC configuration key constants for use in config maps
const (
	OIDCConfigKeyAuthServerURL = "authn-server-url"
	OIDCConfigKeyClientID      = "client-id"
)

// ChainEntry represents a single authenticator in the chain
type ChainEntry struct {
	Type string `mapstructure:"type"`

	// Enable controls whether this authenticator is enabled at all (optional, defaults to true).
	Enable *bool `mapstructure:"enable"`

	// Transport controls per-protocol enablement (optional).
	// If omitted, defaults to enabled for both HTTP and gRPC.
	Transport *Transport `mapstructure:"transport"`

	Config map[string]interface{} `mapstructure:"config"`
}

type Transport struct {
	HTTP *bool `mapstructure:"http"`
	GRPC *bool `mapstructure:"grpc"`
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

	// Prefer new format: if new format is present, use it and ignore old formats
	if o.Authenticator != nil {
		cfg.Authenticator = &AuthenticatorConfig{
			Type:  o.Authenticator.Type,
			Chain: make([]ChainEntry, len(o.Authenticator.Chain)),
		}

		for i, entry := range o.Authenticator.Chain {
			// ChainEntryOptions and ChainEntry intentionally have identical fields, so use a direct conversion.
			cfg.Authenticator.Chain[i] = ChainEntry(entry)
		}
		return cfg
	}

	// Backwards compatibility: convert old format to new format (only if new format not present)
	if o.AllowUnauthenticated != nil && *o.AllowUnauthenticated {
		// Convert legacy allow-unauthenticated: true to new format
		cfg.Authenticator = &AuthenticatorConfig{
			Type: "first_match",
			Chain: []ChainEntry{
				{
					Type:   "allow-unauthenticated",
					Config: nil,
				},
			},
		}
		return cfg
	}

	// Backwards compatibility: convert old oidc format to new format
	if o.OIDC != nil {
		// Convert legacy oidc config to new format
		oidcConfig := make(map[string]interface{})
		if o.OIDC.AuthorizationServerURL != "" {
			oidcConfig[OIDCConfigKeyAuthServerURL] = o.OIDC.AuthorizationServerURL
		}
		if o.OIDC.ClientId != "" {
			oidcConfig[OIDCConfigKeyClientID] = o.OIDC.ClientId
		}
		if o.OIDC.InsecureClient {
			oidcConfig["insecure-client"] = o.OIDC.InsecureClient
		}
		if o.OIDC.SkipClientIDCheck {
			oidcConfig["skip-client-id-check"] = o.OIDC.SkipClientIDCheck
		}
		if o.OIDC.EnforceAudCheck {
			oidcConfig["enforce-aud-check"] = o.OIDC.EnforceAudCheck
		}
		if o.OIDC.SkipIssuerCheck {
			oidcConfig["skip-issuer-check"] = o.OIDC.SkipIssuerCheck
		}
		if o.OIDC.PrincipalUserDomain != "" {
			oidcConfig["principal-user-domain"] = o.OIDC.PrincipalUserDomain
		}

		cfg.Authenticator = &AuthenticatorConfig{
			Type: "first_match",
			Chain: []ChainEntry{
				{
					Type:   "oidc",
					Config: oidcConfig,
				},
			},
		}
		return cfg
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
	Type        string
	EnabledHTTP bool // true if enabled for HTTP, false if disabled
	EnabledGRPC bool // true if enabled for gRPC, false if disabled
	OIDCConfig  *oidc.CompletedConfig
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
	enabledHTTPCount := 0
	enabledGRPCCount := 0
	for _, chainConfig := range chainConfigs {
		if chainConfig.EnabledHTTP {
			enabledHTTPCount++
		}
		if chainConfig.EnabledGRPC {
			enabledGRPCCount++
		}
	}
	if enabledHTTPCount == 0 {
		errs = append(errs, &ConfigError{
			Message: "at least one authenticator in the chain must be enabled for http",
		})
	}
	if enabledGRPCCount == 0 {
		errs = append(errs, &ConfigError{
			Message: "at least one authenticator in the chain must be enabled for grpc",
		})
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
	// Determine effective enablement per protocol.
	// Default is enabled for both protocols unless explicitly disabled.
	baseEnabled := true
	if entry.Enable != nil {
		baseEnabled = *entry.Enable
	}

	// Default transport enablement
	transportHTTP := true
	transportGRPC := true

	// If transport block is provided, it controls per-protocol enablement.
	if entry.Transport != nil {
		if entry.Transport.HTTP != nil {
			transportHTTP = *entry.Transport.HTTP
		}
		if entry.Transport.GRPC != nil {
			transportGRPC = *entry.Transport.GRPC
		}
	}

	enableHTTP := baseEnabled && transportHTTP
	enableGRPC := baseEnabled && transportGRPC

	chainConfig := ChainCompletedConfig{
		Type:        entry.Type,
		EnabledHTTP: enableHTTP,
		EnabledGRPC: enableGRPC,
	}

	switch entry.Type {
	case "guest", "allow-unauthenticated", "x-rh-identity":
		// No config needed
		return chainConfig, nil

	case "oidc":
		// If OIDC is disabled for both protocols, skip config completion/validation.
		if !enableHTTP && !enableGRPC {
			return chainConfig, nil
		}
		if entry.Config == nil {
			errs = append(errs, &ConfigError{
				Message: fmt.Sprintf("oidc authenticator requires config at chain index %d", index),
				Type:    entry.Type,
			})
			return chainConfig, errs
		}
		oidcOpts := oidc.NewOptions()
		if clientID, ok := entry.Config[OIDCConfigKeyClientID].(string); ok {
			oidcOpts.ClientId = clientID
		}
		if authServerURL, ok := entry.Config[OIDCConfigKeyAuthServerURL].(string); ok {
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
