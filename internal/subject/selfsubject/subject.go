package selfsubject

import (
	"fmt"

	authnapi "github.com/project-kessel/inventory-api/internal/authn/api"
	pbv1beta1 "github.com/project-kessel/relations-api/api/kessel/relations/v1beta1"
)

// Resolver resolves subject identifiers using an optional self-subject strategy.
type Resolver struct {
	Strategy SelfSubjectStrategy
}

// NewResolver creates a resolver with the provided strategy.
func NewResolver(strategy SelfSubjectStrategy) *Resolver {
	return &Resolver{Strategy: strategy}
}

// SubjectReferenceFromAuthzContext converts an authz context to a v1beta1 SubjectReference.
// This is used by CheckSelf and CheckSelfBulk service implementations.
//
// Namespace logic:
//   - Uses "rbac" namespace for all auth types
func (r *Resolver) SubjectReferenceFromAuthzContext(authzContext authnapi.AuthzContext) (*pbv1beta1.SubjectReference, error) {
	var subjectID string
	if r != nil && r.Strategy != nil {
		if derived, err := r.Strategy.SubjectFromAuthorizationContext(authzContext); err == nil {
			subjectID = derived
		}
	}
	if subjectID == "" {
		return nil, fmt.Errorf("subject not found")
	}

	namespace := "rbac"

	return &pbv1beta1.SubjectReference{
		Relation: nil, // No relation for direct subject reference
		Subject: &pbv1beta1.ObjectReference{
			Type: &pbv1beta1.ObjectType{
				Namespace: namespace,
				Name:      "principal",
			},
			Id: subjectID,
		},
	}, nil
}
