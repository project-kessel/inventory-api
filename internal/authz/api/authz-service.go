package api

import (
	"context"

	kessel "github.com/project-kessel/relations-api/api/kessel/relations/v1beta1"
)

type Authorizer interface {
	Check(context.Context, *kessel.CheckRequest) (*kessel.CheckResponse, error)
	CreateTuples(context.Context, *kessel.CreateTuplesRequest) (*kessel.CreateTuplesResponse, error)
	DeleteTuples(context.Context, *kessel.DeleteTuplesRequest) (*kessel.DeleteTuplesResponse, error)
	KesselStatus(context.Context) bool
}
