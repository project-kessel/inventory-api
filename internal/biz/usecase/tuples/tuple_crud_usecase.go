package tuples

import (
	"context"
	"fmt"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	authnapi "github.com/project-kessel/inventory-api/internal/authn/api"
	"github.com/project-kessel/inventory-api/internal/biz/model"
	"github.com/project-kessel/inventory-api/internal/biz/usecase/metaauthorizer"
)

// TupleCrudUseCase handles deprecated tuple-layer operations for RBAC backward compatibility.
// This usecase exists only for RBAC and should not be extended. All methods are deprecated.
type TupleCrudUseCase struct {
	Authz          model.RelationsRepository
	MetaAuthorizer metaauthorizer.MetaAuthorizer
	Log            *log.Helper
}

// New creates a new TupleCrudUseCase.
func New(authz model.RelationsRepository, metaAuthorizer metaauthorizer.MetaAuthorizer, logger log.Logger) *TupleCrudUseCase {
	return &TupleCrudUseCase{
		Authz:          authz,
		MetaAuthorizer: metaAuthorizer,
		Log:            log.NewHelper(logger),
	}
}

// CreateTuples creates relationship tuples (DEPRECATED).
// This endpoint exists only for RBAC backward compatibility.
func (uc *TupleCrudUseCase) CreateTuples(ctx context.Context, cmd CreateTuplesCommand) (*CreateTuplesResult, error) {
	startTime := time.Now()

	uc.Log.Info("DEPRECATED: CreateTuples called - this endpoint is for RBAC-only backward compatibility")

	if err := metaauthorizer.EnforceMetaAuthzObject(ctx, uc.MetaAuthorizer, metaauthorizer.RelationCreateTuples, metaauthorizer.NewTupleSystem()); err != nil {
		return nil, err
	}

	// Get authz context for logging
	authzCtx, ok := authnapi.FromAuthzContext(ctx)
	principal := "unknown"
	if ok && authzCtx.Subject != nil {
		if authzCtx.Subject.ClientID != "" {
			principal = string(authzCtx.Subject.ClientID)
		} else if authzCtx.Subject.SubjectId != "" {
			principal = string(authzCtx.Subject.SubjectId)
		}
	}

	var fencing *model.FencingCheck
	if cmd.FencingCheck != nil {
		fc := model.NewFencingCheck(cmd.FencingCheck.LockId, cmd.FencingCheck.LockToken)
		fencing = &fc
	}

	result, err := uc.Authz.CreateTuples(ctx, cmd.Tuples, cmd.Upsert, fencing)
	if err != nil {
		// CRUD operation failed - SEC-MON-REQ-1 compliance (#1 pii_manipulation, #11 warnings_or_errors)
		uc.Log.Warnw("msg", "Create tuples failed",
			"action", "CREATE",
			"resource_type", "tuple",
			"resource_id", fmt.Sprintf("batch_%d_tuples", len(cmd.Tuples)),
			"principal", principal,
			"outcome", "failure",
			"duration_ms", time.Since(startTime).Milliseconds(),
			"reason", err.Error(),
		)
		return nil, err
	}

	// CRUD operation - SEC-MON-REQ-1 compliance (#1 pii_manipulation)
	uc.Log.Infow("msg", "Tuples created",
		"action", "CREATE",
		"resource_type", "tuple",
		"resource_id", fmt.Sprintf("batch_%d_tuples", len(cmd.Tuples)),
		"principal", principal,
		"outcome", "success",
		"duration_ms", time.Since(startTime).Milliseconds(),
	)

	return &CreateTuplesResult{
		ConsistencyToken: result.ConsistencyToken(),
	}, nil
}

// DeleteTuples deletes relationship tuples (DEPRECATED).
// This endpoint exists only for RBAC backward compatibility.
func (uc *TupleCrudUseCase) DeleteTuples(ctx context.Context, cmd DeleteTuplesCommand) (*DeleteTuplesResult, error) {
	startTime := time.Now()

	uc.Log.Info("DEPRECATED: DeleteTuples called - this endpoint is for RBAC-only backward compatibility")

	if err := metaauthorizer.EnforceMetaAuthzObject(ctx, uc.MetaAuthorizer, metaauthorizer.RelationDeleteTuples, metaauthorizer.NewTupleSystem()); err != nil {
		return nil, err
	}

	// Get authz context for logging
	authzCtx, ok := authnapi.FromAuthzContext(ctx)
	principal := "unknown"
	if ok && authzCtx.Subject != nil {
		if authzCtx.Subject.ClientID != "" {
			principal = string(authzCtx.Subject.ClientID)
		} else if authzCtx.Subject.SubjectId != "" {
			principal = string(authzCtx.Subject.SubjectId)
		}
	}

	var fencing *model.FencingCheck
	if cmd.FencingCheck != nil {
		fc := model.NewFencingCheck(cmd.FencingCheck.LockId, cmd.FencingCheck.LockToken)
		fencing = &fc
	}

	result, err := uc.Authz.DeleteTuples(ctx, cmd.Filter, fencing)
	if err != nil {
		// CRUD operation failed - SEC-MON-REQ-1 compliance (#1 pii_manipulation, #11 warnings_or_errors)
		uc.Log.Warnw("msg", "Delete tuples failed",
			"action", "DELETE",
			"resource_type", "tuple",
			"resource_id", "filtered_delete",
			"principal", principal,
			"outcome", "failure",
			"duration_ms", time.Since(startTime).Milliseconds(),
			"reason", err.Error(),
		)
		return nil, err
	}

	// CRUD operation - SEC-MON-REQ-1 compliance (#1 pii_manipulation)
	uc.Log.Infow("msg", "Tuples deleted",
		"action", "DELETE",
		"resource_type", "tuple",
		"resource_id", "filtered_delete",
		"principal", principal,
		"outcome", "success",
		"duration_ms", time.Since(startTime).Milliseconds(),
	)

	return &DeleteTuplesResult{
		ConsistencyToken: result.ConsistencyToken(),
	}, nil
}

// ReadTuples reads relationship tuples via streaming (DEPRECATED).
// This endpoint exists only for RBAC backward compatibility.
func (uc *TupleCrudUseCase) ReadTuples(ctx context.Context, cmd ReadTuplesCommand) (model.ResultStream[model.ReadTuplesItem], error) {
	uc.Log.Info("DEPRECATED: ReadTuples called - this endpoint is for RBAC-only backward compatibility")

	if err := metaauthorizer.EnforceMetaAuthzObject(ctx, uc.MetaAuthorizer, metaauthorizer.RelationReadTuples, metaauthorizer.NewTupleSystem()); err != nil {
		return nil, err
	}

	return uc.Authz.ReadTuples(ctx, cmd.Filter, cmd.Pagination, cmd.Consistency)
}

// AcquireLock acquires a distributed lock (DEPRECATED).
// This endpoint exists only for RBAC backward compatibility.
func (uc *TupleCrudUseCase) AcquireLock(ctx context.Context, cmd AcquireLockCommand) (*AcquireLockResult, error) {
	uc.Log.Info("DEPRECATED: AcquireLock called - this endpoint is for RBAC-only backward compatibility")

	if err := metaauthorizer.EnforceMetaAuthzObject(ctx, uc.MetaAuthorizer, metaauthorizer.RelationAcquireLock, metaauthorizer.NewTupleSystem()); err != nil {
		return nil, err
	}

	result, err := uc.Authz.AcquireLock(ctx, cmd.LockId)
	if err != nil {
		return nil, err
	}

	return &AcquireLockResult{
		LockToken: result.LockToken(),
	}, nil
}
