package api

import (
	"net/http"
)

type Decision string

const (
	Allow  Decision = "ALLOW"
	Deny            = "DENY"
	Ignore          = "IGNORE"
)

type Authenticator interface {

	// TODO: update the return value so it can carry a message in the DENY case
	Authenticate(r *http.Request) (*Identity, Decision)
}
