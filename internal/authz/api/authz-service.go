package api

import (
	"context"

	inv "github.com/project-kessel/inventory-api/api/kessel/inventory/v1beta1/relations"
	kesselv1 "github.com/project-kessel/relations-api/api/kessel/relations/v1"
	kessel "github.com/project-kessel/relations-api/api/kessel/relations/v1beta1"
)

type Authorizer interface {
	Health(ctx context.Context) (*kesselv1.GetReadyzResponse, error)
	CheckForView(context.Context, *inv.CheckForViewRequest) (*inv.CheckForViewResponse, error)
	CheckForUpdate(context.Context, *inv.CheckForUpdateRequest) (*inv.CheckForUpdateResponse, error)
	CheckForCreate(context.Context, *inv.CheckForCreateRequest) (*inv.CheckForCreateResponse, error)
	CreateTuples(context.Context, *kessel.CreateTuplesRequest) (*kessel.CreateTuplesResponse, error)
	DeleteTuples(context.Context, *kessel.DeleteTuplesRequest) (*kessel.DeleteTuplesResponse, error)
	UnsetWorkspace(context.Context, string, string, string) (*kessel.DeleteTuplesResponse, error)
	SetWorkspace(context.Context, string, string, string, string) (*kessel.CreateTuplesResponse, error)
}
