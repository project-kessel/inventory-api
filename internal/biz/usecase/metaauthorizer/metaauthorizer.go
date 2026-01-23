package metaauthorizer

import (
	"context"

	authnapi "github.com/project-kessel/inventory-api/internal/authn/api"
	"github.com/project-kessel/inventory-api/internal/biz/model"
)

// AuthzContext carries authentication/transport context into authorization decisions.
// Alias to authn/api to keep a single source of truth.
type AuthzContext = authnapi.AuthzContext

// MetaAuthorizer provides a simplified authorization check interface for usecases.
// The object and relation are serialized string forms consistent with resource_service.
type MetaAuthorizer interface {
	Check(ctx context.Context, object model.RelationsResource, relation Relation, authzCtx AuthzContext) (bool, error)
}
