package allow

import (
	"context"
	kesselv1 "github.com/project-kessel/relations-api/api/kessel/relations/v1"

	"github.com/go-kratos/kratos/v2/log"
	kessel "github.com/project-kessel/relations-api/api/kessel/relations/v1beta1"
)

type AllowAllAuthz struct {
	Logger *log.Helper
}

func New(logger *log.Helper) *AllowAllAuthz {
	logger.Info("Using authorizer: allow-all")
	return &AllowAllAuthz{
		Logger: logger,
	}
}

func (a *AllowAllAuthz) Health(ctx context.Context) (*kesselv1.GetReadyzResponse, error) {
	return &kesselv1.GetReadyzResponse{Status: "OK", Code: 200}, nil

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
