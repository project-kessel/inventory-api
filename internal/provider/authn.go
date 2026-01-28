package provider

import (
	"context"
	"fmt"
	"net/http"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/transport"

	"github.com/project-kessel/inventory-api/internal/authn/aggregator"
	authnapi "github.com/project-kessel/inventory-api/internal/authn/api"
	"github.com/project-kessel/inventory-api/internal/authn/oidc"
	"github.com/project-kessel/inventory-api/internal/authn/unauthenticated"
	"github.com/project-kessel/inventory-api/internal/authn/util"
	"github.com/project-kessel/inventory-api/internal/authn/xrhidentity"
)

// OIDC configuration key constants for use in config maps.
const (
	OIDCConfigKeyAuthServerURL = "authn-server-url"
	OIDCConfigKeyClientID      = "client-id"
)

// AuthnResult contains the results of authentication initialization.
type AuthnResult struct {
	Authenticator authnapi.Authenticator
}

// OIDCCompletedConfig contains the completed OIDC configuration.
type OIDCCompletedConfig struct {
	ClientId               string
	AuthorizationServerURL string
	InsecureClient         bool
	SkipClientIDCheck      bool
	EnforceAudCheck        bool
	SkipIssuerCheck        bool
	PrincipalUserDomain    string
	Client                 *http.Client
}

// toOIDCCompletedConfig converts to the oidc package's CompletedConfig type.
func (c *OIDCCompletedConfig) toOIDCCompletedConfig() oidc.CompletedConfig {
	opts := &oidc.Options{
		ClientId:               c.ClientId,
		AuthorizationServerURL: c.AuthorizationServerURL,
		InsecureClient:         c.InsecureClient,
		SkipClientIDCheck:      c.SkipClientIDCheck,
		EnforceAudCheck:        c.EnforceAudCheck,
		SkipIssuerCheck:        c.SkipIssuerCheck,
		PrincipalUserDomain:    c.PrincipalUserDomain,
	}
	cfg := oidc.NewConfig(opts)
	cfg.Client = c.Client
	completed, _ := cfg.Complete()
	return completed
}

// ChainCompletedEntry contains completed configuration for a chain entry.
type ChainCompletedEntry struct {
	Type        string
	EnabledHTTP bool
	EnabledGRPC bool
	OIDCConfig  *OIDCCompletedConfig
}

// AuthnCompletedConfig contains the completed authentication configuration.
type AuthnCompletedConfig struct {
	StrategyType aggregator.StrategyType
	ChainConfigs []ChainCompletedEntry
}

// NewAuthenticator creates a new authenticator from options.
func NewAuthenticator(opts *AuthnOptions, logger *log.Helper) (*AuthnResult, error) {
	if errs := opts.Validate(); len(errs) > 0 {
		return nil, fmt.Errorf("authn validation failed: %v", errs)
	}

	completedConfig, errs := completeAuthnConfig(opts)
	if len(errs) > 0 {
		return nil, fmt.Errorf("authn completion failed: %v", errs)
	}

	httpAuth, err := newAuthenticatorForProtocol(completedConfig, protocolHTTP, logger)
	if err != nil {
		return nil, err
	}
	grpcAuth, err := newAuthenticatorForProtocol(completedConfig, protocolGRPC, logger)
	if err != nil {
		return nil, err
	}

	return &AuthnResult{
		Authenticator: &protocolRoutingAuthenticator{
			http: httpAuth,
			grpc: grpcAuth,
		},
	}, nil
}

type protocol string

const (
	protocolHTTP protocol = "http"
	protocolGRPC protocol = "grpc"
)

// protocolRoutingAuthenticator dispatches to a protocol-specific authenticator
// based on the transport.Kind() of the incoming request.
type protocolRoutingAuthenticator struct {
	http authnapi.Authenticator
	grpc authnapi.Authenticator
}

func (a *protocolRoutingAuthenticator) Authenticate(ctx context.Context, t transport.Transporter) (*authnapi.Identity, authnapi.Decision) {
	switch t.Kind() {
	case transport.KindHTTP:
		return a.http.Authenticate(ctx, t)
	case transport.KindGRPC:
		return a.grpc.Authenticate(ctx, t)
	default:
		return nil, authnapi.Deny
	}
}

func completeAuthnConfig(opts *AuthnOptions) (*AuthnCompletedConfig, []error) {
	var errs []error

	// Convert options to normalized chain format
	chain := normalizeAuthnChain(opts)
	if chain == nil {
		return nil, []error{fmt.Errorf("authenticator configuration is required")}
	}

	// Validate strategy type
	strategyType := aggregator.StrategyType(chain.Type)
	if strategyType != aggregator.FirstMatch {
		errs = append(errs, fmt.Errorf("invalid authenticator strategy type: %s", chain.Type))
		return nil, errs
	}

	// Process chain entries
	chainConfigs := make([]ChainCompletedEntry, 0, len(chain.Chain))
	for i, entry := range chain.Chain {
		chainConfig, entryErrs := completeChainEntry(entry, i)
		if len(entryErrs) > 0 {
			errs = append(errs, entryErrs...)
			continue
		}
		chainConfigs = append(chainConfigs, chainConfig)
	}

	// Validate that at least one authenticator is enabled per protocol
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
		errs = append(errs, fmt.Errorf("at least one authenticator in the chain must be enabled for http"))
	}
	if enabledGRPCCount == 0 {
		errs = append(errs, fmt.Errorf("at least one authenticator in the chain must be enabled for grpc"))
	}
	if len(errs) > 0 {
		return nil, errs
	}

	return &AuthnCompletedConfig{
		StrategyType: strategyType,
		ChainConfigs: chainConfigs,
	}, nil
}

// normalizedChain is the internal representation of the authenticator chain.
type normalizedChain struct {
	Type  string
	Chain []normalizedChainEntry
}

type normalizedChainEntry struct {
	Type      string
	Enable    *bool
	Transport *AuthnTransportOptions
	Config    map[string]interface{}
}

// normalizeAuthnChain converts options to a normalized chain format,
// handling backwards compatibility with legacy formats.
func normalizeAuthnChain(opts *AuthnOptions) *normalizedChain {
	// Prefer new format
	if opts.Authenticator != nil {
		chain := &normalizedChain{
			Type:  opts.Authenticator.Type,
			Chain: make([]normalizedChainEntry, len(opts.Authenticator.Chain)),
		}
		for i, entry := range opts.Authenticator.Chain {
			chain.Chain[i] = normalizedChainEntry{
				Type:      entry.Type,
				Enable:    entry.Enable,
				Transport: entry.Transport,
				Config:    entry.Config,
			}
		}
		return chain
	}

	// Backwards compatibility: convert old allow-unauthenticated format
	if opts.AllowUnauthenticated != nil && *opts.AllowUnauthenticated {
		return &normalizedChain{
			Type: "first_match",
			Chain: []normalizedChainEntry{
				{
					Type:   "allow-unauthenticated",
					Config: nil,
				},
			},
		}
	}

	// Backwards compatibility: convert old oidc format
	if opts.OIDC != nil {
		oidcConfig := make(map[string]interface{})
		if opts.OIDC.AuthorizationServerURL != "" {
			oidcConfig[OIDCConfigKeyAuthServerURL] = opts.OIDC.AuthorizationServerURL
		}
		if opts.OIDC.ClientId != "" {
			oidcConfig[OIDCConfigKeyClientID] = opts.OIDC.ClientId
		}
		if opts.OIDC.InsecureClient {
			oidcConfig["insecure-client"] = opts.OIDC.InsecureClient
		}
		if opts.OIDC.SkipClientIDCheck {
			oidcConfig["skip-client-id-check"] = opts.OIDC.SkipClientIDCheck
		}
		if opts.OIDC.EnforceAudCheck {
			oidcConfig["enforce-aud-check"] = opts.OIDC.EnforceAudCheck
		}
		if opts.OIDC.SkipIssuerCheck {
			oidcConfig["skip-issuer-check"] = opts.OIDC.SkipIssuerCheck
		}
		if opts.OIDC.PrincipalUserDomain != "" {
			oidcConfig["principal-user-domain"] = opts.OIDC.PrincipalUserDomain
		}

		return &normalizedChain{
			Type: "first_match",
			Chain: []normalizedChainEntry{
				{
					Type:   "oidc",
					Config: oidcConfig,
				},
			},
		}
	}

	return nil
}

func completeChainEntry(entry normalizedChainEntry, index int) (ChainCompletedEntry, []error) {
	var errs []error

	// Determine effective enablement per protocol
	baseEnabled := true
	if entry.Enable != nil {
		baseEnabled = *entry.Enable
	}

	transportHTTP := true
	transportGRPC := true
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

	chainConfig := ChainCompletedEntry{
		Type:        entry.Type,
		EnabledHTTP: enableHTTP,
		EnabledGRPC: enableGRPC,
	}

	switch entry.Type {
	case "guest", "allow-unauthenticated", "x-rh-identity":
		// No config needed
		return chainConfig, nil

	case "oidc":
		// If OIDC is disabled for both protocols, skip config completion
		if !enableHTTP && !enableGRPC {
			return chainConfig, nil
		}
		if entry.Config == nil {
			errs = append(errs, fmt.Errorf("oidc authenticator requires config at chain index %d", index))
			return chainConfig, errs
		}

		oidcOpts := NewOIDCOptions()
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

		chainConfig.OIDCConfig = &OIDCCompletedConfig{
			ClientId:               oidcOpts.ClientId,
			AuthorizationServerURL: oidcOpts.AuthorizationServerURL,
			InsecureClient:         oidcOpts.InsecureClient,
			SkipClientIDCheck:      oidcOpts.SkipClientIDCheck,
			EnforceAudCheck:        oidcOpts.EnforceAudCheck,
			SkipIssuerCheck:        oidcOpts.SkipIssuerCheck,
			PrincipalUserDomain:    oidcOpts.PrincipalUserDomain,
			Client:                 util.NewClient(oidcOpts.InsecureClient),
		}

	default:
		errs = append(errs, fmt.Errorf("unknown authenticator type at chain index %d: %s", index, entry.Type))
		return chainConfig, errs
	}

	return chainConfig, nil
}

func newAuthenticatorForProtocol(config *AuthnCompletedConfig, p protocol, logger *log.Helper) (authnapi.Authenticator, error) {
	// Create the aggregating authenticator based on strategy type
	var aggregatingAuth aggregator.AggregatingAuthenticator
	switch config.StrategyType {
	case aggregator.FirstMatch:
		firstMatch := aggregator.NewFirstMatch()
		firstMatch.SetLogger(logger)
		aggregatingAuth = firstMatch
	default:
		return nil, fmt.Errorf("unknown authenticator strategy type: %s", config.StrategyType)
	}

	// Create authenticators from chain configs
	authenticatorsAdded := 0
	for i, chainConfig := range config.ChainConfigs {
		// Skip authenticators disabled for this protocol
		switch p {
		case protocolHTTP:
			if !chainConfig.EnabledHTTP {
				logger.Infof("Skipping authenticator '%s' disabled for http at chain index %d", chainConfig.Type, i)
				continue
			}
		case protocolGRPC:
			if !chainConfig.EnabledGRPC {
				logger.Infof("Skipping authenticator '%s' disabled for grpc at chain index %d", chainConfig.Type, i)
				continue
			}
		default:
			return nil, fmt.Errorf("unknown protocol: %s", p)
		}

		var auth authnapi.Authenticator
		var err error

		switch chainConfig.Type {
		case "oidc":
			if chainConfig.OIDCConfig == nil {
				return nil, fmt.Errorf("oidc authenticator requires config at chain index %d", i)
			}
			logger.Infof("Loading OIDC info from %s", chainConfig.OIDCConfig.AuthorizationServerURL)
			auth, err = oidc.New(chainConfig.OIDCConfig.toOIDCCompletedConfig())
			if err != nil {
				return nil, fmt.Errorf("failed to create oidc authenticator: %w", err)
			}

		case "guest", "allow-unauthenticated":
			logger.Info("Allowing unauthenticated access")
			auth = unauthenticated.New()

		case "x-rh-identity":
			logger.Info("Will check for x-rh-identity header")
			auth = xrhidentity.New()

		default:
			return nil, fmt.Errorf("unknown authenticator type in chain at index %d: %s", i, chainConfig.Type)
		}

		aggregatingAuth.Add(auth)
		authenticatorsAdded++
	}

	if authenticatorsAdded == 0 {
		return nil, fmt.Errorf("no authenticators were successfully created or enabled in the authentication chain")
	}

	return aggregatingAuth, nil
}
