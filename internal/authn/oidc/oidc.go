// package oidc provides an Authenticator based on OAuth2 OIDC JWTs.
package oidc

import (
	"context"

	coreosoidc "github.com/coreos/go-oidc/v3/oidc"
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
		return nil, err
	}

	oidcConfig := &coreosoidc.Config{ClientID: c.ClientId}
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
		return nil, api.Deny
	}

	// TODO: make JWT claim fields configurable
	// extract the claims we care about
	u := &Claims{}
	err = tok.Claims(u)
	if err != nil {
		return nil, api.Deny
	}
	if u.Id == "" {
		return nil, api.Deny
	}

	if u.Audience != o.CompletedConfig.ClientId {
		return nil, api.Deny
	}

	// TODO: What are the tenant and group claims?
	return &api.Identity{Principal: u.Id}, api.Allow
}

// TODO: make JWT claim fields configurable
// Claims holds the values we want to extract from the JWT.
type Claims struct {
	Id       string `json:"preferred_username"`
	Audience string `json:"aud"`
}

func (l *OAuth2Authenticator) Verify(token string) (*coreosoidc.IDToken, error) {
	return l.Verifier.Verify(l.ClientContext, token)
}
