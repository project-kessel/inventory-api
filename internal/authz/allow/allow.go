package allow

import (
	"context"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/project-kessel/inventory-api/internal/biz/model"
	kesselv1 "github.com/project-kessel/relations-api/api/kessel/relations/v1"
	"github.com/project-kessel/relations-api/api/kessel/relations/v1beta1"
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

func (a *AllowAllAuthz) Check(context.Context, string, string, *model.Resource, *v1beta1.SubjectReference) (v1beta1.CheckResponse_Allowed, *v1beta1.ConsistencyToken, error) {
	return v1beta1.CheckResponse_ALLOWED_TRUE, nil, nil
}

func (a *AllowAllAuthz) CheckForUpdate(context.Context, string, string, *model.Resource, *v1beta1.SubjectReference) (v1beta1.CheckForUpdateResponse_Allowed, *v1beta1.ConsistencyToken, error) {
	return v1beta1.CheckForUpdateResponse_ALLOWED_TRUE, nil, nil
}

func (a *AllowAllAuthz) CreateTuples(ctx context.Context, r *v1beta1.CreateTuplesRequest) (*v1beta1.CreateTuplesResponse, error) {
	return &v1beta1.CreateTuplesResponse{}, nil
}

func (a *AllowAllAuthz) DeleteTuples(ctx context.Context, r *v1beta1.DeleteTuplesRequest) (*v1beta1.DeleteTuplesResponse, error) {
	return &v1beta1.DeleteTuplesResponse{}, nil
}

func (a *AllowAllAuthz) UnsetWorkspace(ctx context.Context, local_resource_id, name, namespace string) (*v1beta1.DeleteTuplesResponse, error) {
	return &v1beta1.DeleteTuplesResponse{}, nil
}

func (a *AllowAllAuthz) SetWorkspace(ctx context.Context, local_resource_id, workspace, name, namespace string) (*v1beta1.CreateTuplesResponse, error) {
	return &v1beta1.CreateTuplesResponse{}, nil
}
