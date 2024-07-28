package clientcert

import (
	"net/http"

	"github.com/project-kessel/inventory-api/internal/authn/api"
)

type ClientCertAuthenticator struct{}

func New() *ClientCertAuthenticator {
	return &ClientCertAuthenticator{}
}

func (a *ClientCertAuthenticator) Authenticate(r *http.Request) (*api.Identity, api.Decision) {
	if r.TLS == nil {
		return nil, api.Ignore
	}

	if len(r.TLS.PeerCertificates) == 0 {
		return nil, api.Ignore
	}

	cert := r.TLS.PeerCertificates[0]

	// TODO: What do we do about tenant id here?
	// TODO: Should we say all reporters will authenticate with client certificates?
	// TODO: How do we get Type, Href, etc.?  We may need to look at a CR in openshift or something if that's
	// how reporters get registered.
	return &api.Identity{
		Principal:  cert.Subject.CommonName,
		Groups:     cert.Subject.Organization,
		IsReporter: true,
	}, api.Allow
}
