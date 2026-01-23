package metaauthorizer

import (
	"context"

	authnapi "github.com/project-kessel/inventory-api/internal/authn/api"
	kessel "github.com/project-kessel/relations-api/api/kessel/relations/v1beta1"
)

// AuthzContext carries authentication/transport context into authorization decisions.
// Alias to authn/api to keep a single source of truth.
type AuthzContext = authnapi.AuthzContext

// MetaAuthorizer provides a simplified authorization check interface for usecases.
// The subject and relation are serialized string forms consistent with resource_service.
type MetaAuthorizer interface {
	Check(ctx context.Context, subject *kessel.SubjectReference, object *kessel.ObjectReference, relation string, authzCtx AuthzContext) (bool, error)
}
