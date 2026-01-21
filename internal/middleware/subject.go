package middleware

import (
	"strings"

	authnapi "github.com/project-kessel/inventory-api/internal/authn/api"
	pbv1beta1 "github.com/project-kessel/relations-api/api/kessel/relations/v1beta1"
)

// SubjectIDFromIdentity extracts the subject ID from an identity.
// This is shared logic used by both CheckSelf/CheckSelfBulk and meta-authorization.
//
// Conversion logic:
//   - x-rh-identity: Uses UserID if available, otherwise Principal
//   - OIDC: Parses Principal (extracts subject from "domain/subject" format)
func SubjectIDFromIdentity(identity *authnapi.Identity) string {
	if identity.AuthType == "x-rh-identity" {
		// For x-rh-identity, prefer UserID if available (more stable identifier)
		if identity.UserID != "" {
			return identity.UserID
		}
		if identity.Principal != "" {
			return identity.Principal
		}
		// Fallback: should not happen for authenticated requests
		return identity.Principal
	}

	// For OIDC and other auth types, parse Principal
	// Principal might be in "domain/subject" format (OIDC) or just "subject"
	subjectID := identity.Principal
	if parts := strings.SplitN(identity.Principal, "/", 2); len(parts) == 2 {
		subjectID = parts[1]
	}
	return subjectID
}

// SubjectReferenceFromIdentity converts identity to a v1beta1 SubjectReference.
// This is used by CheckSelf and CheckSelfBulk service implementations.
//
// Namespace logic:
//   - x-rh-identity: Always uses "rbac" namespace
//   - OIDC/other: Uses "rbac" by default, but can use identity.Type if set
func SubjectReferenceFromIdentity(identity *authnapi.Identity) *pbv1beta1.SubjectReference {
	subjectID := SubjectIDFromIdentity(identity)

	// Determine namespace
	// For x-rh-identity: Type field contains "User", "System", etc. but we use "rbac" as namespace
	// For OIDC: Type is typically empty, default to "rbac"
	namespace := "rbac"
	if identity.AuthType != "x-rh-identity" && identity.Type != "" {
		// For non-x-rh-identity auth types, use Type if set
		namespace = identity.Type
	}

	return &pbv1beta1.SubjectReference{
		Relation: nil, // No relation for direct subject reference
		Subject: &pbv1beta1.ObjectReference{
			Type: &pbv1beta1.ObjectType{
				Namespace: namespace,
				Name:      "principal",
			},
			Id: subjectID,
		},
	}
}
