package metaauthorizer

import (
	"context"

	authnapi "github.com/project-kessel/inventory-api/internal/authn/api"
)

type Relation string

const RelationCheckSelf Relation = "check_self"
const RelationLookupResources Relation = "lookup_resources"
const RelationReportResource Relation = "report_resource"
const RelationCheckForUpdate Relation = "check_for_update"
const RelationDeleteResource Relation = "delete_resource"
const RelationCheck Relation = "check"
const RelationCheckBulk Relation = "check_bulk"
const RelationCheckSelfBulk Relation = "check_self_bulk"
const RelationCheckForUpdateBulk Relation = "check_for_update_bulk"
const RelationCheckForUpdateSelfBulk Relation = "check_for_update_self_bulk"

// SimpleMetaAuthorizer implements the current decision rules:
// - HTTP + x-rh-identity: allow only if relation == "check_self"
// - gRPC: deny if relation == "check_self", allow otherwise
// - deny otherwise
type SimpleMetaAuthorizer struct{}

func NewSimpleMetaAuthorizer() *SimpleMetaAuthorizer {
	return &SimpleMetaAuthorizer{}
}

func (s *SimpleMetaAuthorizer) Check(_ context.Context, _ MetaObject, relation Relation, authzCtx authnapi.AuthzContext) (bool, error) {
	if authzCtx.Protocol == authnapi.ProtocolUnknown {
		return false, nil
	}
	if authzCtx.Protocol == authnapi.ProtocolGRPC {
		return relation != RelationCheckSelf, nil
	}
	if authzCtx.IsAuthenticated() && authzCtx.Subject.AuthType == authnapi.AuthTypeXRhIdentity {
		return relation == RelationCheckSelf, nil
	}
	return false, nil
}
