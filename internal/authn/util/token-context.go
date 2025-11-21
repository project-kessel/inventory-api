package util

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"strings"

	coreosoidc "github.com/coreos/go-oidc/v3/oidc"
)

type tokenContextKey struct{}
type rawTokenContextKey struct{}

// NewTokenContext stores the OIDC ID token in the context.
// This allows the token to be retrieved later for extracting claims like client_id.
func NewTokenContext(ctx context.Context, token *coreosoidc.IDToken) context.Context {
	return context.WithValue(ctx, tokenContextKey{}, token)
}

// NewRawTokenContext stores the raw JWT token string in the context.
// This is used as a fallback when we can't store the verified IDToken.
func NewRawTokenContext(ctx context.Context, rawToken string) context.Context {
	return context.WithValue(ctx, rawTokenContextKey{}, rawToken)
}

// FromTokenContext retrieves the OIDC ID token from the context.
// Returns the token and true if found, nil and false otherwise.
func FromTokenContext(ctx context.Context) (*coreosoidc.IDToken, bool) {
	token, ok := ctx.Value(tokenContextKey{}).(*coreosoidc.IDToken)
	return token, ok
}

// FromRawTokenContext retrieves the raw JWT token string from the context.
// Returns the token and true if found, empty string and false otherwise.
func FromRawTokenContext(ctx context.Context) (string, bool) {
	token, ok := ctx.Value(rawTokenContextKey{}).(string)
	return token, ok
}

// DecodeJWTClaims decodes the JWT payload to extract claims.
// This is safe when the token has already been verified by the authenticator.
func DecodeJWTClaims(rawToken string) (map[string]interface{}, error) {
	parts := strings.Split(rawToken, ".")
	if len(parts) != 3 {
		return nil, nil
	}

	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, err
	}

	var claims map[string]interface{}
	if err := json.Unmarshal(payload, &claims); err != nil {
		return nil, err
	}

	return claims, nil
}
