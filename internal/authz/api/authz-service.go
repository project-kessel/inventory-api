package api

import (
	"context"
	kesselv1 "github.com/project-kessel/relations-api/api/kessel/relations/v1"
	kessel "github.com/project-kessel/relations-api/api/kessel/relations/v1beta1"
)

type Authorizer interface {
	Health(ctx context.Context) (*kesselv1.GetReadyzResponse, error)
	Check(context.Context, *kessel.CheckRequest) (*kessel.CheckResponse, error)
	CreateTuples(context.Context, *kessel.CreateTuplesRequest) (*kessel.CreateTuplesResponse, error)
	DeleteTuples(context.Context, *kessel.DeleteTuplesRequest) (*kessel.DeleteTuplesResponse, error)
	SetWorkspace(context.Context, string, string, string, string) (*kessel.CreateTuplesResponse, error)
}
