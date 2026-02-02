package api

type SubjectId string

type OrganizationId string

type Issuer string

type ClientID string

type AuthType string

const (
	AuthTypeOIDC                 AuthType = "oidc"
	AuthTypeXRhIdentity          AuthType = "x-rh-identity"
	AuthTypeAllowUnauthenticated AuthType = "allow-unauthenticated"
)

// Claims represent the JWT-aligned identity claims for the requester.
type Claims struct {
	SubjectId      SubjectId      `json:"sub" yaml:"sub"`
	OrganizationId OrganizationId `json:"org_id" yaml:"org_id"`
	Issuer         Issuer         `json:"iss" yaml:"iss"`
	ClientID       ClientID       `json:"client_id" yaml:"client_id"`
	AuthType       AuthType       `json:"auth_type" yaml:"auth_type"`
}

// UnauthenticatedClaims returns a sentinel claims value for unauthenticated requests.
func UnauthenticatedClaims() *Claims {
	return &Claims{AuthType: AuthTypeAllowUnauthenticated}
}

// IsAuthenticated reports whether claims represent an authenticated request.
func (c *Claims) IsAuthenticated() bool {
	return c != nil && c.AuthType != AuthTypeAllowUnauthenticated
}
