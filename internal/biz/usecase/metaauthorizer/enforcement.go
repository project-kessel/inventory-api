package metaauthorizer

import (
	"context"
	"errors"

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
		return ErrMetaAuthorizationDenied
	}
	return nil
}
