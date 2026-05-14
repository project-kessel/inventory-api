package middleware

import (
	"context"
	"fmt"

	"github.com/go-kratos/kratos/v2/errors"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/transport"

	authnapi "github.com/project-kessel/inventory-api/internal/authn/api"
)

// validateAuthDecision checks if the authentication decision allows access.
// Returns an error if access should be denied, or nil if allowed.
// This consolidates the decision validation logic shared between unary middleware
// and stream interceptors.
func validateAuthDecision(decision authnapi.Decision, claims *authnapi.Claims) error {
	if decision == authnapi.Deny {
		return errors.Unauthorized(reason, "Authentication denied")
	}
	if decision == authnapi.Ignore {
		return errors.Unauthorized(reason, "No valid authentication found")
	}
	if decision != authnapi.Allow {
		return errors.Unauthorized(reason, fmt.Sprintf("Authentication failed with decision: %s", decision))
	}
	// Defensive check: claims should not be nil when decision is Allow
	// but we check to prevent panics if an authenticator implementation violates the contract
	if claims == nil {
		return errors.Unauthorized(reason, "Invalid claims: authenticator returned Allow with nil claims")
	}
	return nil
}

// logAuthenticationFailure logs authentication failures for security monitoring.
func logAuthenticationFailure(ctx context.Context, decision authnapi.Decision, reason string) {
	logger := log.NewHelper(log.DefaultLogger)

	endpoint := "unknown"
	if t, ok := transport.FromServerContext(ctx); ok {
		endpoint = t.Operation()
	}

	// Auth failure - SEC-MON-REQ-1 compliance (#7 invalid_login)
	logger.Warnw(
		"event", "authentication_failure",
		"endpoint", endpoint,
		"reason", reason,
		"decision", string(decision),
		"outcome", "failure",
	)
}
