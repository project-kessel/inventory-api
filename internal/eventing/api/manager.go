package api

import (
	"context"

	authnapi "github.com/project-kessel/inventory-api/internal/authn/api"
)

type Manager interface {
	Lookup(identity *authnapi.Identity, resource_type string, resource_id int64) (Producer, error)
	Errs() <-chan error
	Shutdown(ctx context.Context) error
}
