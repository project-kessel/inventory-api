package clientcert

import (
	"context"
	"crypto/x509"

	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/peer"

	"github.com/go-kratos/kratos/v2/transport"
	kgrpc "github.com/go-kratos/kratos/v2/transport/grpc"
	khttp "github.com/go-kratos/kratos/v2/transport/http"

	"github.com/project-kessel/inventory-api/internal/authn/api"
)

type ClientCertAuthenticator struct{}

func New() *ClientCertAuthenticator {
	return &ClientCertAuthenticator{}
}

func (a *ClientCertAuthenticator) Authenticate(ctx context.Context, t transport.Transporter) (*api.Identity, api.Decision) {
	var cert *x509.Certificate

	switch t := t.(type) {
	case *khttp.Transport:
		tr := t
		r := tr.Request()

		if r.TLS == nil {
			return nil, api.Ignore
		}

		if len(r.TLS.PeerCertificates) == 0 {
			return nil, api.Ignore
		}

		cert = r.TLS.PeerCertificates[0]

	case *kgrpc.Transport:
		p, ok := peer.FromContext(ctx)
		if !ok {
			return nil, api.Ignore
		}

		tlsAuth, ok := p.AuthInfo.(credentials.TLSInfo)
		if !ok {
			return nil, api.Ignore
		}

		if len(tlsAuth.State.VerifiedChains) == 0 || len(tlsAuth.State.VerifiedChains[0]) == 0 {
			return nil, api.Ignore
		}

		cert = tlsAuth.State.VerifiedChains[0][0]
	}

	// TODO: What do we do about tenant id here?
	// TODO: Should we say all reporters will authenticate with client certificates?
	// TODO: How do we get Type, Href, etc.?  We may need to look at a CR in openshift or something if that's
	// how reporters get registered.
	if cert != nil {
		return &api.Identity{
			Principal:  cert.Subject.CommonName,
			Groups:     cert.Subject.Organization,
			IsReporter: true,
		}, api.Allow
	}
	return nil, api.Ignore
}
