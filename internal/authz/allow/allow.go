package allow

import (
	"context"

	kesselv1 "github.com/project-kessel/relations-api/api/kessel/relations/v1"
	"github.com/project-kessel/relations-api/api/kessel/relations/v1beta1"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/project-kessel/inventory-api/internal/biz/model"
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

func (a *AllowAllAuthz) CheckForView(context.Context, string, string, *model.Resource, *v1beta1.SubjectReference) (v1beta1.CheckResponse_Allowed, *v1beta1.ConsistencyToken, error) {
	return v1beta1.CheckResponse_ALLOWED_TRUE, nil, nil
}

func (a *AllowAllAuthz) CheckForUpdate(context.Context, string, string, *model.Resource, *v1beta1.SubjectReference) (v1beta1.CheckForUpdateResponse_Allowed, *v1beta1.ConsistencyToken, error) {
	return v1beta1.CheckForUpdateResponse_ALLOWED_TRUE, nil, nil
}

// this doesn't really make sense in the context of allow?
func (a *AllowAllAuthz) LookupResources(context.Context, string, string, *model.Resource, *v1beta1.SubjectReference) (chan *kessel.ObjectReference, chan *kessel.ConsistencyToken, chan error, error) {
	return make(chan *kessel.ObjectReference), make(chan *kessel.ConsistencyToken), nil, nil
}

func (a *AllowAllAuthz) CreateTuples(ctx context.Context, r *kessel.CreateTuplesRequest) (*kessel.CreateTuplesResponse, error) {
	return &kessel.CreateTuplesResponse{}, nil
}

func (a *AllowAllAuthz) DeleteTuples(ctx context.Context, r *kessel.DeleteTuplesRequest) (*kessel.DeleteTuplesResponse, error) {
	return &kessel.DeleteTuplesResponse{}, nil
}

func (a *AllowAllAuthz) UnsetWorkspace(ctx context.Context, local_resource_id, name, namespace string) (*kessel.DeleteTuplesResponse, error) {
	return &kessel.DeleteTuplesResponse{}, nil
}

func (a *AllowAllAuthz) SetWorkspace(ctx context.Context, local_resource_id, workspace, name, namespace string) (*kessel.CreateTuplesResponse, error) {
	return &kessel.CreateTuplesResponse{}, nil
}
