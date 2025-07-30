package api

import (
	"context"

	"google.golang.org/grpc"

	kesselv1 "github.com/project-kessel/relations-api/api/kessel/relations/v1"
	kessel "github.com/project-kessel/relations-api/api/kessel/relations/v1beta1"

	"github.com/project-kessel/inventory-api/internal/biz/model_legacy"
)

// Authorizer defines the interface for authorization and access control operations.
// It provides methods for checking permissions, managing relationships, and health checks.
type Authorizer interface {
	Health(ctx context.Context) (*kesselv1.GetReadyzResponse, error)
	Check(context.Context, string, string, *model_legacy.Resource, *kessel.SubjectReference) (kessel.CheckResponse_Allowed, *kessel.ConsistencyToken, error)
	CheckForUpdate(context.Context, string, string, *model_legacy.Resource, *kessel.SubjectReference) (kessel.CheckForUpdateResponse_Allowed, *kessel.ConsistencyToken, error)
	LookupResources(ctx context.Context, in *kessel.LookupResourcesRequest) (grpc.ServerStreamingClient[kessel.LookupResourcesResponse], error)
	CreateTuples(context.Context, *kessel.CreateTuplesRequest) (*kessel.CreateTuplesResponse, error)
	DeleteTuples(context.Context, *kessel.DeleteTuplesRequest) (*kessel.DeleteTuplesResponse, error)
	AcquireLock(context.Context, *kessel.AcquireLockRequest) (*kessel.AcquireLockResponse, error)
	UnsetWorkspace(context.Context, string, string, string) (*kessel.DeleteTuplesResponse, error)
	SetWorkspace(context.Context, string, string, string, string, bool) (*kessel.CreateTuplesResponse, error)
}
