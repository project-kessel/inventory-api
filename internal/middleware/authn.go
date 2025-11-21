package middleware

import (
	"context"
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

				identity, decision := authenticator.Authenticate(ctx, t)
				if decision != authnapi.Allow {
					return nil, errors.Unauthorized(reason, "Unauthorized")
				}

				ctx = context.WithValue(ctx, IdentityRequestKey, identity)

				// If we had a token before, preserve it. Otherwise, try to get it from the context
				// that OAuth2Authenticator might have modified (though it won't propagate automatically).
				// As a fallback, extract from headers if OIDC authenticator was used.
				if !hasToken {
					// Check again after Authenticate (in case OAuth2Authenticator stored it)
					if token, hasToken = util.FromTokenContext(ctx); !hasToken {
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
					} else {
						// Token was found in context, preserve it
						ctx = util.NewTokenContext(ctx, token)
					}
				} else {
					// Preserve the token we had before
					ctx = util.NewTokenContext(ctx, token)
				}

				return next(ctx, req)
			}
			return nil, ErrWrongContext
		}
	}
}

var (
	IdentityRequestKey = &contextKey{"authnapi.Identity"}
	GetIdentity        = GetFromContext[authnapi.Identity](IdentityRequestKey)
)
