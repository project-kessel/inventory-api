package api

import (
	"context"

	"github.com/go-kratos/kratos/v2/transport"
)

type Decision string

const (
	Allow  Decision = "ALLOW"
	Deny            = "DENY"
	Ignore          = "IGNORE"
)

type Authenticator interface {

	// TODO: update the return value so it can carry a message in the DENY case
	Authenticate(context.Context, transport.Transporter) (*Identity, Decision)
}
