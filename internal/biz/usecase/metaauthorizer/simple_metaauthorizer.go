package metaauthorizer

import (
	"context"

	authnapi "github.com/project-kessel/inventory-api/internal/authn/api"
	kessel "github.com/project-kessel/relations-api/api/kessel/relations/v1beta1"
)

const RelationCheckSelf = "check_self"

// SimpleMetaAuthorizer implements the current decision rules:
// - HTTP + x-rh-identity: allow only if relation == "check_self"
// - gRPC: deny if relation == "check_self", allow otherwise
// - deny otherwise
type SimpleMetaAuthorizer struct{}

func NewSimpleMetaAuthorizer() *SimpleMetaAuthorizer {
	return &SimpleMetaAuthorizer{}
}

func (s *SimpleMetaAuthorizer) Check(_ context.Context, _ *kessel.SubjectReference, _ *kessel.ObjectReference, relation string, authzCtx authnapi.AuthzContext) (bool, error) {
	if authzCtx.Protocol == authnapi.ProtocolGRPC {
		return relation != RelationCheckSelf, nil
	}
	if authzCtx.Identity != nil && authzCtx.Identity.AuthType == "x-rh-identity" {
		return relation == RelationCheckSelf, nil
	}
	return false, nil
}
