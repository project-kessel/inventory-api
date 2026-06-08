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
	// Subject are optional; some transports may be unauthenticated.
	Subject *Claims
}

// IsAuthenticated reports whether the context carries authenticated claims.
func (a AuthzContext) IsAuthenticated() bool {
	if a.Subject == nil {
		return false
	}
	return a.Subject.IsAuthenticated()
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

// ExtractPrincipal returns the principal identifier from an AuthzContext for logging.
// Returns "unknown" if the context has no authenticated subject.
// Prefers ClientID over SubjectId when both are present.
func (a AuthzContext) ExtractPrincipal() string {
	if a.Subject == nil {
		return "unknown"
	}
	if a.Subject.ClientID != "" {
		return string(a.Subject.ClientID)
	}
	if a.Subject.SubjectId != "" {
		return string(a.Subject.SubjectId)
	}
	return "unknown"
}
