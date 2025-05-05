package allow

import (
	"context"
	"io"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	"github.com/go-kratos/kratos/v2/log"
	kesselv1 "github.com/project-kessel/relations-api/api/kessel/relations/v1"
	"github.com/project-kessel/relations-api/api/kessel/relations/v1beta1"

	"github.com/project-kessel/inventory-api/internal/biz/model"
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

type mockLookupResourcesClient struct {
	ctx context.Context
}

func (m *mockLookupResourcesClient) Recv() (*v1beta1.LookupResourcesResponse, error) {
	// Return EOF immediately to indicate end of stream
	return nil, io.EOF
}

func (m *mockLookupResourcesClient) Header() (metadata.MD, error) {
	return nil, nil
}

func (m *mockLookupResourcesClient) Trailer() metadata.MD {
	return nil
}

func (m *mockLookupResourcesClient) CloseSend() error {
	return nil
}

func (m *mockLookupResourcesClient) Context() context.Context {
	return m.ctx
}

func (m *mockLookupResourcesClient) SendMsg(msg interface{}) error {
	return nil
}

func (m *mockLookupResourcesClient) RecvMsg(msg interface{}) error {
	return nil
}

func (a *AllowAllAuthz) LookupResources(ctx context.Context, in *v1beta1.LookupResourcesRequest) (grpc.ServerStreamingClient[v1beta1.LookupResourcesResponse], error) {
	return &mockLookupResourcesClient{ctx: ctx}, nil
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

func (a *AllowAllAuthz) SetWorkspace(ctx context.Context, local_resource_id, workspace, name, namespace string, upsert bool) (*v1beta1.CreateTuplesResponse, error) {
	return &v1beta1.CreateTuplesResponse{}, nil
}
