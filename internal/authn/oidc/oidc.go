// package oidc provides an Authenticator based on OAuth2 OIDC JWTs.
package oidc

import (
	"context"
	"fmt"
	"regexp"
	"strings"

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

	if c.PrincipalUserDomain == "" {
		c.PrincipalUserDomain = "redhat.com"
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
		return nil, api.Deny
	}

	// TODO: make JWT claim fields configurable
	// extract the claims we care about
	u := &Claims{}
	err = tok.Claims(u)
	if err != nil {
		return nil, api.Deny
	}

	if o.EnforceAudCheck {
		if u.Audience != o.CompletedConfig.ClientId {
			return nil, api.Deny
		}
	}

	if issuerCheck(u.Issuer, o.PrincipalUserDomain) {
		principal := fmt.Sprintf("%s:%s", o.PrincipalUserDomain, u.Subject)
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
}

func (l *OAuth2Authenticator) Verify(token string) (*coreosoidc.IDToken, error) {
	return l.Verifier.Verify(l.ClientContext, token)
}

func issuerCheck(issuer string, domain string) bool {
	domain = strings.ToLower(domain)
	issuer = strings.ToLower(issuer)

	// Define a regex pattern to strip "http://", "https://", and any "/path-uri" part from the Issuer
	re := regexp.MustCompile(`^(https?://)?([^/]+)`)

	// Extract the host part from the input
	match := re.FindStringSubmatch(issuer)
	if len(match) < 3 {
		return false
	}
	// The actual host without the scheme and path
	actualIssuer := strings.ToLower(match[2])

	if domain == actualIssuer {
		return true
	}

	if strings.HasSuffix(actualIssuer, "."+domain) {
		return true
	}
	return false
}
