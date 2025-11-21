// package oidc provides an Authenticator based on OAuth2 OIDC JWTs.
package oidc

import (
	"context"
	"fmt"

	coreosoidc "github.com/coreos/go-oidc/v3/oidc"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/transport"

	"github.com/project-kessel/inventory-api/internal/authn/api"
	"github.com/project-kessel/inventory-api/internal/authn/util"
)

type OAuth2Authenticator struct {
	CompletedConfig

	ClientContext context.Context
	Verifier      *coreosoidc.IDTokenVerifier
}

func New(c CompletedConfig) (*OAuth2Authenticator, error) {
	// this allows us to test locally against KeyCloak or something using an http client that doesn't check
	// serving certs
	ctx := coreosoidc.ClientContext(context.Background(), c.Client)
	provider, err := coreosoidc.NewProvider(ctx, c.AuthorizationServerURL)
	if err != nil {
		return nil, fmt.Errorf("failed to create provider: %w", err)
	}

	if c.PrincipalUserDomain == "" {
		c.PrincipalUserDomain = "localhost"
	}

	oidcConfig := &coreosoidc.Config{ClientID: c.ClientId, SkipClientIDCheck: c.SkipClientIDCheck, SkipIssuerCheck: c.SkipIssuerCheck}
	return &OAuth2Authenticator{
		CompletedConfig: c,
		ClientContext:   ctx,
		Verifier:        provider.Verifier(oidcConfig),
	}, nil

}

func (o *OAuth2Authenticator) Authenticate(ctx context.Context, t transport.Transporter) (*api.Identity, api.Decision) {
	// get the token from the request
	rawToken := util.GetBearerToken(t)

	// ensure we got one
	if rawToken == "" {
		return nil, api.Ignore
	}

	// verify and parse it
	tok, err := o.Verify(rawToken)
	if err != nil {
		log.Errorf("failed to verify the access token: %v", err)
		return nil, api.Deny
	}

	// Store the token in context so it can be accessed later (e.g., for logging client_id)
	ctx = util.NewTokenContext(ctx, tok)

	// TODO: make JWT claim fields configurable
	// extract the claims we care about
	u := &Claims{}
	err = tok.Claims(u)
	if err != nil {
		log.Errorf("failed to extract claims: %v", err)
		return nil, api.Deny
	}

	if o.EnforceAudCheck {
		if u.Audience != o.ClientId {
			log.Debugf("aud does not match the requesting client-id -- decision DENY")
			return nil, api.Deny
		}
	}

	if u.Subject != "" && o.PrincipalUserDomain != "" {
		principal := fmt.Sprintf("%s/%s", o.PrincipalUserDomain, u.Subject)
		return &api.Identity{Principal: principal}, api.Allow
	}

	return nil, api.Deny
}

// TODO: make JWT claim fields configurable
// Claims holds the values we want to extract from the JWT.
type Claims struct {
	Audience          string `json:"aud"`
	Issuer            string `json:"iss"`
	Subject           string `json:"sub"`
	PreferredUsername string `json:"preferred_username"`
	ClientID          string `json:"client_id"`
}

func (l *OAuth2Authenticator) Verify(token string) (*coreosoidc.IDToken, error) {
	return l.Verifier.Verify(l.ClientContext, token)
}
