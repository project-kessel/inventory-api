package api

import "context"

// Protocol identifies the request transport type for authz decisions.
type Protocol string

const (
	ProtocolHTTP    Protocol = "http"
	ProtocolGRPC    Protocol = "grpc"
	ProtocolUnknown Protocol = "unknown"
)

// AuthzContext carries authentication/transport context into authorization decisions.
type AuthzContext struct {
	Protocol Protocol
	// Claims are optional; some transports may be unauthenticated.
	Claims *Claims
}

// IsAuthenticated reports whether the context carries authenticated claims.
func (a AuthzContext) IsAuthenticated() bool {
	if a.Claims == nil {
		return false
	}
	return a.Claims.IsAuthenticated()
}

type authzContextKey struct{}

// NewAuthzContext stores an AuthzContext in the context.
func NewAuthzContext(ctx context.Context, authzCtx AuthzContext) context.Context {
	return context.WithValue(ctx, authzContextKey{}, authzCtx)
}

// FromAuthzContext retrieves an AuthzContext from the context.
func FromAuthzContext(ctx context.Context) (AuthzContext, bool) {
	authzCtx, ok := ctx.Value(authzContextKey{}).(AuthzContext)
	return authzCtx, ok
}
