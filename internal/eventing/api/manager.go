package api

import (
	"context"

	"github.com/google/uuid"

	authnapi "github.com/project-kessel/inventory-api/internal/authn/api"
)

// Manager defines the interface for managing event producers and routing events to appropriate topics.
// It provides methods to look up producers based on identity and resource information,
// error handling, and graceful shutdown capabilities.
type Manager interface {
	// Lookup hides the logic of figuring out which topic to send an event on.
	Lookup(claims *authnapi.Claims, resource_type string, resource_id uuid.UUID) (Producer, error)

	Errs() <-chan error
	Shutdown(ctx context.Context) error
}
