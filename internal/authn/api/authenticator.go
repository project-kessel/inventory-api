package api

import (
	"context"

	"github.com/go-kratos/kratos/v2/transport"
)

// Decision represents the result of an authentication attempt.
type Decision string

const (
	// Allow indicates that the authentication was successful and the request should be allowed.
	Allow Decision = "ALLOW"
	// Deny indicates that the authentication failed and the request should be denied.
	Deny Decision = "DENY"
	// Ignore indicates that the authenticator cannot make a decision and defers to the next authenticator.
	Ignore Decision = "IGNORE"
)

// Authenticator defines the interface for authentication providers.
// Implementations should examine the transport and return claims and decision.
type Authenticator interface {

	// TODO: update the return value so it can carry a message in the DENY case
	// Authenticate examines the transport context and returns claims and authentication decision.
	Authenticate(context.Context, transport.Transporter) (*Claims, Decision)
}
