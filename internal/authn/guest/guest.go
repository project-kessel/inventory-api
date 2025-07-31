package guest

import (
	"context"
	"strings"

	"github.com/go-kratos/kratos/v2/transport"
	kgrpc "github.com/go-kratos/kratos/v2/transport/grpc"
	khttp "github.com/go-kratos/kratos/v2/transport/http"

	"google.golang.org/grpc/peer"

	"github.com/project-kessel/inventory-api/internal/authn/api"
)

// GuestAuthenticator provides guest authentication using the User-Agent header as the principal.
type GuestAuthenticator struct{}

// New creates a new GuestAuthenticator instance.
func New() *GuestAuthenticator {
	return &GuestAuthenticator{}
}

// Authenticate creates a guest identity using the ip-address and always allows the request.
func (a *GuestAuthenticator) Authenticate(ctx context.Context, t transport.Transporter) (*api.Identity, api.Decision) {

	var principal string

	switch tr := t.(type) {
	case *khttp.Transport:
		if addr := tr.Request().RemoteAddr; addr != "" {
			principal = strings.Split(addr, ":")[0]
		}
	case *kgrpc.Transport:
		if p, ok := peer.FromContext(ctx); ok && p.Addr != nil {
			principal = strings.Split(p.Addr.String(), ":")[0]
		}
	}

	if principal == "" {
		principal = "guest"
	}

	identity := &api.Identity{
		Principal: principal,
		IsGuest:   true,
	}

	return identity, api.Allow
}
