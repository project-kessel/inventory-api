package api

import (
	"context"

	authnapi "github.com/project-kessel/inventory-api/internal/authn/api"
	"github.com/project-kessel/inventory-api/internal/models"
)

type Manager interface {
	Lookup(identity *authnapi.Identity, resource *models.Resource) (Producer, error)
	Errs() <-chan error
	Shutdown(ctx context.Context) error
}
