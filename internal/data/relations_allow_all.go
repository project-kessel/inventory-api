package data

import (
	"context"
	"io"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	"github.com/go-kratos/kratos/v2/log"
	kesselv1 "github.com/project-kessel/relations-api/api/kessel/relations/v1"
	"github.com/project-kessel/relations-api/api/kessel/relations/v1beta1"
)

type AllowAllRelationsRepository struct {
	Logger *log.Helper
}

func NewAllowAllRelationsRepository(logger *log.Helper) *AllowAllRelationsRepository {
	logger.Info("Using relations repository: allow-all")
	return &AllowAllRelationsRepository{
		Logger: logger,
	}
}

func (a *AllowAllRelationsRepository) Health(ctx context.Context) (*kesselv1.GetReadyzResponse, error) {
	return &kesselv1.GetReadyzResponse{Status: "OK", Code: 200}, nil

}

func (a *AllowAllRelationsRepository) Check(context.Context, string, string, string, string, string, *v1beta1.SubjectReference) (v1beta1.CheckResponse_Allowed, *v1beta1.ConsistencyToken, error) {
	return v1beta1.CheckResponse_ALLOWED_TRUE, nil, nil
}

func (a *AllowAllRelationsRepository) CheckForUpdate(context.Context, string, string, string, string, *v1beta1.SubjectReference) (v1beta1.CheckForUpdateResponse_Allowed, *v1beta1.ConsistencyToken, error) {
	return v1beta1.CheckForUpdateResponse_ALLOWED_TRUE, nil, nil
}

func (a *AllowAllRelationsRepository) CheckBulk(ctx context.Context, req *v1beta1.CheckBulkRequest) (*v1beta1.CheckBulkResponse, error) {
	pairs := make([]*v1beta1.CheckBulkResponsePair, len(req.Items))
	for i, item := range req.Items {
		pairs[i] = &v1beta1.CheckBulkResponsePair{
			Request: item,
			Response: &v1beta1.CheckBulkResponsePair_Item{
				Item: &v1beta1.CheckBulkResponseItem{
					Allowed: v1beta1.CheckBulkResponseItem_ALLOWED_TRUE,
				},
			},
		}
	}
	return &v1beta1.CheckBulkResponse{Pairs: pairs}, nil
}

func (a *AllowAllRelationsRepository) CheckForUpdateBulk(ctx context.Context, req *v1beta1.CheckForUpdateBulkRequest) (*v1beta1.CheckForUpdateBulkResponse, error) {
	pairs := make([]*v1beta1.CheckBulkResponsePair, len(req.GetItems()))
	for i, item := range req.GetItems() {
		pairs[i] = &v1beta1.CheckBulkResponsePair{
			Request: item,
			Response: &v1beta1.CheckBulkResponsePair_Item{
				Item: &v1beta1.CheckBulkResponseItem{
					Allowed: v1beta1.CheckBulkResponseItem_ALLOWED_TRUE,
				},
			},
		}
	}
	return &v1beta1.CheckForUpdateBulkResponse{Pairs: pairs}, nil
}

type allowAllLookupResourcesClient struct {
	ctx context.Context
}

func (m *allowAllLookupResourcesClient) Recv() (*v1beta1.LookupResourcesResponse, error) {
	return nil, io.EOF
}

func (m *allowAllLookupResourcesClient) Header() (metadata.MD, error) {
	return nil, nil
}

func (m *allowAllLookupResourcesClient) Trailer() metadata.MD {
	return nil
}

func (m *allowAllLookupResourcesClient) CloseSend() error {
	return nil
}

func (m *allowAllLookupResourcesClient) Context() context.Context {
	return m.ctx
}

func (m *allowAllLookupResourcesClient) SendMsg(msg interface{}) error {
	return nil
}

func (m *allowAllLookupResourcesClient) RecvMsg(msg interface{}) error {
	return nil
}

func (a *AllowAllRelationsRepository) LookupResources(ctx context.Context, in *v1beta1.LookupResourcesRequest) (grpc.ServerStreamingClient[v1beta1.LookupResourcesResponse], error) {
	return &allowAllLookupResourcesClient{ctx: ctx}, nil
}

type allowAllLookupSubjectsClient struct {
	ctx context.Context
}

func (m *allowAllLookupSubjectsClient) Recv() (*v1beta1.LookupSubjectsResponse, error) {
	return nil, io.EOF
}

func (m *allowAllLookupSubjectsClient) Header() (metadata.MD, error) {
	return nil, nil
}

func (m *allowAllLookupSubjectsClient) Trailer() metadata.MD {
	return nil
}

func (m *allowAllLookupSubjectsClient) CloseSend() error {
	return nil
}

func (m *allowAllLookupSubjectsClient) Context() context.Context {
	return m.ctx
}

func (m *allowAllLookupSubjectsClient) SendMsg(msg interface{}) error {
	return nil
}

func (m *allowAllLookupSubjectsClient) RecvMsg(msg interface{}) error {
	return nil
}

func (a *AllowAllRelationsRepository) LookupSubjects(ctx context.Context, in *v1beta1.LookupSubjectsRequest) (grpc.ServerStreamingClient[v1beta1.LookupSubjectsResponse], error) {
	return &allowAllLookupSubjectsClient{ctx: ctx}, nil
}

func (a *AllowAllRelationsRepository) AcquireLock(ctx context.Context, r *v1beta1.AcquireLockRequest) (*v1beta1.AcquireLockResponse, error) {
	return &v1beta1.AcquireLockResponse{}, nil
}

func (a *AllowAllRelationsRepository) CreateTuples(ctx context.Context, r *v1beta1.CreateTuplesRequest) (*v1beta1.CreateTuplesResponse, error) {
	return &v1beta1.CreateTuplesResponse{}, nil
}

func (a *AllowAllRelationsRepository) DeleteTuples(ctx context.Context, r *v1beta1.DeleteTuplesRequest) (*v1beta1.DeleteTuplesResponse, error) {
	return &v1beta1.DeleteTuplesResponse{}, nil
}

type allowAllReadTuplesClient struct {
	ctx context.Context
}

func (m *allowAllReadTuplesClient) Recv() (*v1beta1.ReadTuplesResponse, error) {
	return nil, io.EOF
}

func (m *allowAllReadTuplesClient) Header() (metadata.MD, error) {
	return nil, nil
}

func (m *allowAllReadTuplesClient) Trailer() metadata.MD {
	return nil
}

func (m *allowAllReadTuplesClient) CloseSend() error {
	return nil
}

func (m *allowAllReadTuplesClient) Context() context.Context {
	return m.ctx
}

func (m *allowAllReadTuplesClient) SendMsg(msg interface{}) error {
	return nil
}

func (m *allowAllReadTuplesClient) RecvMsg(msg interface{}) error {
	return nil
}

func (a *AllowAllRelationsRepository) ReadTuples(ctx context.Context, r *v1beta1.ReadTuplesRequest) (grpc.ServerStreamingClient[v1beta1.ReadTuplesResponse], error) {
	return &allowAllReadTuplesClient{ctx: ctx}, nil
}

func (a *AllowAllRelationsRepository) UnsetWorkspace(ctx context.Context, local_resource_id, name, namespace string) (*v1beta1.DeleteTuplesResponse, error) {
	return &v1beta1.DeleteTuplesResponse{}, nil
}

func (a *AllowAllRelationsRepository) SetWorkspace(ctx context.Context, local_resource_id, workspace, name, namespace string, upsert bool) (*v1beta1.CreateTuplesResponse, error) {
	return &v1beta1.CreateTuplesResponse{}, nil
}
