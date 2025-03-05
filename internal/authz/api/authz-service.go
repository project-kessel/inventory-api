package api

import (
	"context"

	"github.com/project-kessel/inventory-api/internal/biz/model"
	kesselv1 "github.com/project-kessel/relations-api/api/kessel/relations/v1"
	kessel "github.com/project-kessel/relations-api/api/kessel/relations/v1beta1"
)

type Authorizer interface {
	Health(ctx context.Context) (*kesselv1.GetReadyzResponse, error)
	CheckForView(context.Context, string, string, *model.Resource, *kessel.SubjectReference) (kessel.CheckResponse_Allowed, *kessel.ConsistencyToken, error)
	CheckForUpdate(context.Context, string, string, *model.Resource, *kessel.SubjectReference) (kessel.CheckForUpdateResponse_Allowed, *kessel.ConsistencyToken, error)
	CreateTuples(context.Context, *kessel.CreateTuplesRequest) (*kessel.CreateTuplesResponse, error)
	DeleteTuples(context.Context, *kessel.DeleteTuplesRequest) (*kessel.DeleteTuplesResponse, error)
	UnsetWorkspace(context.Context, string, string, string) (*kessel.DeleteTuplesResponse, error)
	SetWorkspace(context.Context, string, string, string, string) (*kessel.CreateTuplesResponse, error)
}
