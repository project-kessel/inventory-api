package metaauthorizer

import (
	"context"
	"errors"

	"github.com/go-kratos/kratos/v2/log"
	authnapi "github.com/project-kessel/inventory-api/internal/authn/api"
)

var (
	// ErrMetaAuthorizerUnavailable indicates the meta authorizer is not configured.
	ErrMetaAuthorizerUnavailable = errors.New("meta authorizer unavailable")
	// ErrMetaAuthorizationDenied indicates the meta authorization check failed.
	ErrMetaAuthorizationDenied = errors.New("meta authorization denied")
	// ErrMetaAuthzContextMissing indicates missing authz context in request.
	ErrMetaAuthzContextMissing = errors.New("meta authorization context missing")
)

// EnforceMetaAuthzObject performs meta-authorization using a MetaObject.
// This is the standard enforcement function used across all usecases.
func EnforceMetaAuthzObject(ctx context.Context, authorizer MetaAuthorizer, relation Relation, metaObject MetaObject, logger log.Logger) error {
	logHelper := log.NewHelper(logger)

	authzCtx, ok := authnapi.FromAuthzContext(ctx)
	if !ok {
		// Auth failure - SEC-MON-REQ-1 compliance (#8 authorization_failure)
		logHelper.Warnw(
			"event", "authorization_failure",
			"reason", "missing auth context",
			"outcome", "failure",
		)
		return ErrMetaAuthzContextMissing
	}
	if authorizer == nil {
		return ErrMetaAuthorizerUnavailable
	}

	allowed, err := authorizer.Check(ctx, metaObject, relation, authzCtx)
	if err != nil {
		return err
	}
	if !allowed {
		// Build principal from authzCtx
		principal := buildPrincipal(authzCtx)

		// Extract resource info from metaObject
		resourceType, resourceId := extractResourceInfo(metaObject)

		// Auth failure - SEC-MON-REQ-1 compliance (#8 authorization_failure, possibly #1 or #2)
		logHelper.Warnw(
			"event", "authorization_failure",
			"principal", principal,
			"resource_type", resourceType,
			"resource_id", resourceId,
			"required_permission", string(relation),
			"reason", "permission denied",
			"outcome", "failure",
		)
		return ErrMetaAuthorizationDenied
	}
	return nil
}

// buildPrincipal constructs a principal identifier from the authorization context.
func buildPrincipal(authzCtx AuthzContext) string {
	if authzCtx.Subject == nil {
		return "unknown"
	}
	if authzCtx.Subject.ClientID != "" {
		return "client_id:" + string(authzCtx.Subject.ClientID)
	}
	if authzCtx.Subject.OrganizationId != "" && authzCtx.Subject.SubjectId != "" {
		return string(authzCtx.Subject.OrganizationId) + ":" + string(authzCtx.Subject.SubjectId)
	}
	return "unknown"
}

// extractResourceInfo extracts resource type and ID from a MetaObject for logging.
func extractResourceInfo(metaObject MetaObject) (string, string) {
	switch obj := metaObject.(type) {
	case InventoryResource:
		return obj.ResourceType().String(), obj.LocalResourceId().String()
	case ResourceTypeRef:
		return obj.ResourceType().String(), ""
	case TupleSystem:
		return "tuple_system", ""
	default:
		return "unknown", ""
	}
}
