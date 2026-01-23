package selfsubject

import (
	authnapi "github.com/project-kessel/inventory-api/internal/authn/api"
)

// SelfSubjectStrategy produces a subject identifier for self-access decisions.
type SelfSubjectStrategy interface {
	SubjectFromAuthorizationContext(authzContext authnapi.AuthzContext) (string, error)
}
