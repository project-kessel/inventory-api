package allow

import (
	"context"

	kesselv1 "github.com/project-kessel/relations-api/api/kessel/relations/v1"

	"github.com/go-kratos/kratos/v2/log"
	pb "github.com/project-kessel/inventory-api/api/kessel/inventory/v1beta1/relations"
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

func (a *AllowAllAuthz) CheckForView(ctx context.Context, r *pb.CheckForViewRequest) (*pb.CheckForViewResponse, error) {
	return &pb.CheckForViewResponse{
		Allowed: pb.CheckForViewResponse_ALLOWED_TRUE,
	}, nil
}

func (a *AllowAllAuthz) CheckForUpdate(ctx context.Context, r *pb.CheckForUpdateRequest) (*pb.CheckForUpdateResponse, error) {
	return &pb.CheckForUpdateResponse{
		Allowed: pb.CheckForUpdateResponse_ALLOWED_TRUE,
	}, nil
}

func (a *AllowAllAuthz) CheckForCreate(ctx context.Context, r *pb.CheckForCreateRequest) (*pb.CheckForCreateResponse, error) {
	return &pb.CheckForCreateResponse{
		Allowed: pb.CheckForCreateResponse_ALLOWED_TRUE,
	}, nil
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
