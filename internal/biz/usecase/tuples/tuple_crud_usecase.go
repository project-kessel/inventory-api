package tuples

import (
	"context"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/project-kessel/inventory-api/internal/biz/model"
	"github.com/project-kessel/inventory-api/internal/biz/usecase/metaauthorizer"
	relationspb "github.com/project-kessel/relations-api/api/kessel/relations/v1beta1"
	"google.golang.org/grpc"
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
	uc.Log.Info("DEPRECATED: CreateTuples called - this endpoint is for RBAC-only backward compatibility")

	// Meta-authorization check
	if err := metaauthorizer.EnforceMetaAuthzObject(ctx, uc.MetaAuthorizer, metaauthorizer.RelationCreateTuples, metaauthorizer.NewTupleSystem()); err != nil {
		return nil, err
	}

	// Convert command to relations-api v1beta1 request
	relReq := createTuplesCommandToV1beta1(cmd)

	// Delegate to authorizer
	resp, err := uc.Authz.CreateTuples(ctx, relReq)
	if err != nil {
		return nil, err
	}

	return &CreateTuplesResult{
		ConsistencyToken: model.DeserializeConsistencyToken(resp.GetConsistencyToken().GetToken()),
	}, nil
}

// DeleteTuples deletes relationship tuples (DEPRECATED).
// This endpoint exists only for RBAC backward compatibility.
func (uc *TupleCrudUseCase) DeleteTuples(ctx context.Context, cmd DeleteTuplesCommand) (*DeleteTuplesResult, error) {
	uc.Log.Info("DEPRECATED: DeleteTuples called - this endpoint is for RBAC-only backward compatibility")

	// Meta-authorization check
	if err := metaauthorizer.EnforceMetaAuthzObject(ctx, uc.MetaAuthorizer, metaauthorizer.RelationDeleteTuples, metaauthorizer.NewTupleSystem()); err != nil {
		return nil, err
	}

	// Convert command to relations-api v1beta1 request
	relReq := deleteTuplesCommandToV1beta1(cmd)

	resp, err := uc.Authz.DeleteTuples(ctx, relReq)
	if err != nil {
		return nil, err
	}

	return &DeleteTuplesResult{
		ConsistencyToken: model.DeserializeConsistencyToken(resp.GetConsistencyToken().GetToken()),
	}, nil
}

// ReadTuples reads relationship tuples via streaming (DEPRECATED).
// This endpoint exists only for RBAC backward compatibility.
// Note: Returns relations-api stream directly (see LookupResourcesCommand pattern).
func (uc *TupleCrudUseCase) ReadTuples(ctx context.Context, cmd ReadTuplesCommand) (grpc.ServerStreamingClient[relationspb.ReadTuplesResponse], error) {
	uc.Log.Info("DEPRECATED: ReadTuples called - this endpoint is for RBAC-only backward compatibility")

	// Meta-authorization check
	if err := metaauthorizer.EnforceMetaAuthzObject(ctx, uc.MetaAuthorizer, metaauthorizer.RelationReadTuples, metaauthorizer.NewTupleSystem()); err != nil {
		return nil, err
	}

	// Convert command to relations-api v1beta1 request
	relReq := readTuplesCommandToV1beta1(cmd)

	return uc.Authz.ReadTuples(ctx, relReq)
}

// AcquireLock acquires a distributed lock (DEPRECATED).
// This endpoint exists only for RBAC backward compatibility.
func (uc *TupleCrudUseCase) AcquireLock(ctx context.Context, cmd AcquireLockCommand) (*AcquireLockResult, error) {
	uc.Log.Info("DEPRECATED: AcquireLock called - this endpoint is for RBAC-only backward compatibility")

	// Meta-authorization check
	if err := metaauthorizer.EnforceMetaAuthzObject(ctx, uc.MetaAuthorizer, metaauthorizer.RelationAcquireLock, metaauthorizer.NewTupleSystem()); err != nil {
		return nil, err
	}

	// Convert command to relations-api v1beta1 request
	relReq := acquireLockCommandToV1beta1(cmd)

	resp, err := uc.Authz.AcquireLock(ctx, relReq)
	if err != nil {
		return nil, err
	}

	return &AcquireLockResult{
		LockToken: resp.GetLockToken(),
	}, nil
}
