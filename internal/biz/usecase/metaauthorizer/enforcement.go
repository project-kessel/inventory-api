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
func EnforceMetaAuthzObject(ctx context.Context, authorizer MetaAuthorizer, relation Relation, metaObject MetaObject) error {
	authzCtx, ok := authnapi.FromAuthzContext(ctx)
	if !ok {
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
		// Extract principal from authzCtx
		principal := "unknown"
		if authzCtx.Subject != nil {
			if authzCtx.Subject.ClientID != "" {
				principal = string(authzCtx.Subject.ClientID)
			} else if authzCtx.Subject.SubjectId != "" {
				principal = string(authzCtx.Subject.SubjectId)
			}
		}

		// Extract resource information from MetaObject
		resourceType := "unknown"
		resourceId := "unknown"
		switch obj := metaObject.(type) {
		case InventoryResource:
			resourceType = obj.ResourceType().String()
			resourceId = string(obj.LocalResourceId())
		case ResourceTypeRef:
			resourceType = obj.ResourceType().String()
			resourceId = "type_level_operation"
		case TupleSystem:
			resourceType = "tuple_system"
			resourceId = "system"
		}

		// Auth failure - SEC-MON-REQ-1 compliance (#8 authorization_failure, #1 pii_manipulation)
		logger := log.NewHelper(log.DefaultLogger)
		logger.Warnw("msg", "Permission denied",
			"event", "authorization_failure",
			"action", "authorize_resource_access",
			"principal", principal,
			"resource_type", resourceType,
			"resource_id", resourceId,
			"required_permission", string(relation),
			"outcome", "failure",
		)

		return ErrMetaAuthorizationDenied
	}
	return nil
}
