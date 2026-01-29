package selfsubject

import (
	authnapi "github.com/project-kessel/inventory-api/internal/authn/api"
	"github.com/project-kessel/inventory-api/internal/biz/model"
)

// SelfSubjectStrategy produces a SubjectReference for self-access decisions.
type SelfSubjectStrategy interface {
	SubjectFromAuthorizationContext(authzContext authnapi.AuthzContext) (model.SubjectReference, error)
}
