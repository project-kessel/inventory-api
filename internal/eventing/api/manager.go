package api

import (
	"context"
	"github.com/google/uuid"

	authnapi "github.com/project-kessel/inventory-api/internal/authn/api"
)

type Manager interface {
	// Lookup hides the logic of figuring out which topic to send an event on.
	Lookup(identity *authnapi.Identity, resource_type string, resource_id uuid.UUID) (Producer, error)

	Errs() <-chan error
	Shutdown(ctx context.Context) error
}
