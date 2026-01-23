package middleware

import (
	"context"
	"fmt"
	"strings"

	"github.com/go-kratos/kratos/v2/errors"
	"github.com/go-kratos/kratos/v2/middleware"
	"github.com/go-kratos/kratos/v2/transport"

	authnapi "github.com/project-kessel/inventory-api/internal/authn/api"
	"github.com/project-kessel/inventory-api/internal/authn/util"
)

const (
	reason string = "UNAUTHORIZED"
)

var (
	ErrWrongContext = errors.Unauthorized(reason, "Wrong context for middleware")
)

func Authentication(authenticator authnapi.Authenticator) func(middleware.Handler) middleware.Handler {
	return func(next middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req any) (any, error) {
			if t, ok := transport.FromServerContext(ctx); ok {
				// Check if token is already in context (from a previous middleware or interceptor)
				token, hasToken := util.FromTokenContext(ctx)

				claims, decision := authenticator.Authenticate(ctx, t)
				if decision == authnapi.Deny {
					return nil, errors.Unauthorized(reason, "Authentication denied")
				} else if decision == authnapi.Ignore {
					return nil, errors.Unauthorized(reason, "No valid authentication found")
				} else if decision != authnapi.Allow {
					// Handle any unexpected decision values
					return nil, errors.Unauthorized(reason, fmt.Sprintf("Authentication failed with decision: %s", decision))
				}

				// Defensive check: claims should not be nil when decision is Allow
				// but we check to prevent panics if an authenticator implementation violates the contract
				if claims == nil {
					return nil, errors.Unauthorized(reason, "Invalid claims: authenticator returned Allow with nil claims")
				}

				ctx = context.WithValue(ctx, ClaimsRequestKey, claims)
				ctx = EnsureAuthzContext(ctx, claims)

				// Try to get token from context if we don't have it yet
				// (in case OAuth2Authenticator stored it during Authenticate)
				if !hasToken {
					token, hasToken = util.FromTokenContext(ctx)
				}

				if hasToken {
					// Preserve the token we found
					ctx = util.NewTokenContext(ctx, token)
				} else {
					// Fallback: extract from headers (token was already verified by authenticator)
					authHeader := t.RequestHeader().Get("Authorization")
					if authHeader != "" {
						parts := strings.SplitN(authHeader, " ", 2)
						if len(parts) == 2 && strings.EqualFold(parts[0], "bearer") {
							rawToken := parts[1]
							// Store the raw token string so GetClientIDFromContext can decode it
							ctx = util.NewRawTokenContext(ctx, rawToken)
						}
					}
				}

				return next(ctx, req)
			}
			return nil, ErrWrongContext
		}
	}
}

// EnsureAuthzContext populates the authz context with claims and protocol if missing.
func EnsureAuthzContext(ctx context.Context, claims *authnapi.Claims) context.Context {
	if _, ok := authnapi.FromAuthzContext(ctx); ok {
		return ctx
	}

	protocol := authnapi.ProtocolUnknown
	if t, ok := transport.FromServerContext(ctx); ok {
		switch t.Kind() {
		case transport.KindHTTP:
			protocol = authnapi.ProtocolHTTP
		case transport.KindGRPC:
			protocol = authnapi.ProtocolGRPC
		default:
			// leave as ProtocolUnknown to allow MetaAuthorizer to fail closed
			protocol = authnapi.ProtocolUnknown
		}
	}

	return authnapi.NewAuthzContext(ctx, authnapi.AuthzContext{
		Protocol: protocol,
		Claims:   claims,
	})
}

var (
	ClaimsRequestKey = &contextKey{"authnapi.Claims"}
	GetClaims        = GetFromContext[authnapi.Claims](ClaimsRequestKey)
)
