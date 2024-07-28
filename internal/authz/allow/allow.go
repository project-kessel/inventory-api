package allow

import (
	"context"

	kessel "github.com/project-kessel/relations-api/api/kessel/relations/v1beta1"
)

type AllowAllAuthz struct{}

func New() *AllowAllAuthz {
	return &AllowAllAuthz{}
}

func (a *AllowAllAuthz) Check(ctx context.Context, r *kessel.CheckRequest) (*kessel.CheckResponse, error) {
	return &kessel.CheckResponse{
		Allowed: kessel.CheckResponse_ALLOWED_TRUE,
	}, nil
}

func (a *AllowAllAuthz) CreateTuples(ctx context.Context, r *kessel.CreateTuplesRequest) (*kessel.CreateTuplesResponse, error) {
	return &kessel.CreateTuplesResponse{}, nil
}

func (a *AllowAllAuthz) DeleteTuples(ctx context.Context, r *kessel.DeleteTuplesRequest) (*kessel.DeleteTuplesResponse, error) {
	return &kessel.DeleteTuplesResponse{}, nil
}
