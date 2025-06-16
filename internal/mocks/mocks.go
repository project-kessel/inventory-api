package mocks

import (
	"context"
	"io"

	"github.com/google/uuid"
	"github.com/project-kessel/inventory-api/internal/pubsub"
	"google.golang.org/grpc/metadata"

	pb "github.com/project-kessel/inventory-api/api/kessel/inventory/v1"

	"github.com/confluentinc/confluent-kafka-go/v2/kafka"

	kesselv1 "github.com/project-kessel/relations-api/api/kessel/relations/v1"
	"github.com/project-kessel/relations-api/api/kessel/relations/v1beta1"
	"github.com/stretchr/testify/mock"
	"google.golang.org/grpc"

	"github.com/project-kessel/inventory-api/internal/biz/model"
)

type MockHealthRepo struct {
	mock.Mock
}

type MockAuthz struct {
	mock.Mock
}

type MockConsumer struct {
	mock.Mock
}

type MockedReporterResourceRepository struct {
	mock.Mock
}

type MockedInventoryResourceRepository struct {
	mock.Mock
}

type MockedListenManager struct {
	mock.Mock
}

type MockedSubscription struct {
	mock.Mock
}

type MockLookupResourcesStream struct {
	mock.Mock
	Responses []*v1beta1.LookupResourcesResponse
	current   int
}

func (m *MockAuthz) Health(ctx context.Context) (*kesselv1.GetReadyzResponse, error) {
	args := m.Called(ctx)
	return args.Get(0).(*kesselv1.GetReadyzResponse), args.Error(1)
}

func (m *MockHealthRepo) IsBackendAvailable(ctx context.Context) (*pb.GetReadyzResponse, error) {
	args := m.Called(ctx)
	return args.Get(0).(*pb.GetReadyzResponse), args.Error(1)
}

func (m *MockHealthRepo) IsRelationsAvailable(ctx context.Context) (*pb.GetReadyzResponse, error) {
	args := m.Called(ctx)
	return args.Get(0).(*pb.GetReadyzResponse), args.Error(1)

}

func (m *MockAuthz) Check(ctx context.Context, namespace string, permission string, res *model.Resource, sub *v1beta1.SubjectReference) (v1beta1.CheckResponse_Allowed, *v1beta1.ConsistencyToken, error) {
	args := m.Called(ctx, namespace, permission, res, sub)
	return args.Get(0).(v1beta1.CheckResponse_Allowed), args.Get(1).(*v1beta1.ConsistencyToken), args.Error(2)
}

func (m *MockAuthz) CheckForUpdate(ctx context.Context, namespace string, permission string, res *model.Resource, sub *v1beta1.SubjectReference) (v1beta1.CheckForUpdateResponse_Allowed, *v1beta1.ConsistencyToken, error) {
	args := m.Called(ctx, namespace, permission, res, sub)
	return args.Get(0).(v1beta1.CheckForUpdateResponse_Allowed), args.Get(1).(*v1beta1.ConsistencyToken), args.Error(2)
}

func (m *MockAuthz) CreateTuples(ctx context.Context, req *v1beta1.CreateTuplesRequest) (*v1beta1.CreateTuplesResponse, error) {
	args := m.Called(ctx, req)
	return args.Get(0).(*v1beta1.CreateTuplesResponse), args.Error(1)
}

func (m *MockAuthz) DeleteTuples(ctx context.Context, request *v1beta1.DeleteTuplesRequest) (*v1beta1.DeleteTuplesResponse, error) {
	args := m.Called(ctx, request)
	return args.Get(0).(*v1beta1.DeleteTuplesResponse), args.Error(1)
}

func (m *MockAuthz) UnsetWorkspace(ctx context.Context, namespace, localResourceId, resourceType string) (*v1beta1.DeleteTuplesResponse, error) {
	args := m.Called(ctx, namespace, localResourceId, resourceType)
	return args.Get(0).(*v1beta1.DeleteTuplesResponse), args.Error(1)
}

func (m *MockAuthz) SetWorkspace(ctx context.Context, local_resource_id, workspace, namespace, name string, upsert bool) (*v1beta1.CreateTuplesResponse, error) {
	args := m.Called(ctx, local_resource_id, workspace, namespace, name)
	return args.Get(0).(*v1beta1.CreateTuplesResponse), args.Error(1)
}

// Update the MockAuthz LookupResources method to match the exact signature
func (m *MockAuthz) LookupResources(ctx context.Context, request *v1beta1.LookupResourcesRequest) (grpc.ServerStreamingClient[v1beta1.LookupResourcesResponse], error) {
	args := m.Called(ctx, request)
	return args.Get(0).(grpc.ServerStreamingClient[v1beta1.LookupResourcesResponse]), args.Error(1)
}

func (m *MockConsumer) CommitOffsets(offsets []kafka.TopicPartition) ([]kafka.TopicPartition, error) {
	args := m.Called(offsets)
	return args.Get(0).([]kafka.TopicPartition), args.Error(1)
}

func (m *MockConsumer) SubscribeTopics(topics []string, rebalanceCb kafka.RebalanceCb) (err error) {
	args := m.Called(topics, rebalanceCb)
	return args.Error(0)
}

func (m *MockConsumer) Poll(timeoutMs int) (event kafka.Event) {
	args := m.Called(timeoutMs)
	return args.Get(0).(kafka.Event)
}

func (m *MockConsumer) IsClosed() bool {
	args := m.Called()
	return args.Get(0).(bool)
}

func (m *MockConsumer) Close() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockConsumer) AssignmentLost() bool {
	args := m.Called()
	return args.Get(0).(bool)
}

func (m *MockLookupResourcesStream) Recv() (*v1beta1.LookupResourcesResponse, error) {
	if m.current >= len(m.Responses) {
		return nil, io.EOF
	}
	res := m.Responses[m.current]
	m.current++
	return res, nil
}

func (m *MockLookupResourcesStream) Header() (metadata.MD, error) {
	args := m.Called()
	return args.Get(0).(metadata.MD), args.Error(1)
}

func (m *MockLookupResourcesStream) Trailer() metadata.MD {
	args := m.Called()
	return args.Get(0).(metadata.MD)
}

func (m *MockLookupResourcesStream) CloseSend() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockLookupResourcesStream) Context() context.Context {
	args := m.Called()
	return args.Get(0).(context.Context)
}

func (m *MockLookupResourcesStream) SendMsg(msg interface{}) error {
	args := m.Called(msg)
	return args.Error(0)
}

func (m *MockLookupResourcesStream) RecvMsg(msg interface{}) error {
	args := m.Called(msg)
	return args.Error(0)
}

func (r *MockedReporterResourceRepository) Create(ctx context.Context, resource *model.Resource, namespace string, txid string) (*model.Resource, error) {
	args := r.Called(ctx, resource, namespace, txid)
	return args.Get(0).(*model.Resource), args.Error(1)
}

func (r *MockedReporterResourceRepository) Update(ctx context.Context, resource *model.Resource, id uuid.UUID, namespace string, txid string) (*model.Resource, error) {
	args := r.Called(ctx, resource, id, namespace, txid)
	return args.Get(0).(*model.Resource), args.Error(1)
}

func (r *MockedReporterResourceRepository) Delete(ctx context.Context, id uuid.UUID, namespace string) (*model.Resource, error) {
	args := r.Called(ctx, id, namespace)
	return args.Get(0).(*model.Resource), args.Error(1)
}

func (r *MockedReporterResourceRepository) FindByID(ctx context.Context, id uuid.UUID) (*model.Resource, error) {
	args := r.Called(ctx, id)
	return args.Get(0).(*model.Resource), args.Error(1)
}

func (r *MockedReporterResourceRepository) FindByReporterResourceId(ctx context.Context, id model.ReporterResourceId) (*model.Resource, error) {
	args := r.Called(ctx, id)
	return args.Get(0).(*model.Resource), args.Error(1)
}

func (r *MockedReporterResourceRepository) FindByInventoryIdAndReporter(ctx context.Context, inventoryId *uuid.UUID, reporterResourceId string, reporterType string) (*model.Resource, error) {
	args := r.Called(ctx, inventoryId, reporterResourceId, reporterType)
	return args.Get(0).(*model.Resource), args.Error(1)
}

func (r *MockedReporterResourceRepository) FindByReporterResourceIdv1beta2(ctx context.Context, id model.ReporterResourceUniqueIndex) (*model.Resource, error) {
	args := r.Called(ctx, id)
	return args.Get(0).(*model.Resource), args.Error(1)
}

func (r *MockedReporterResourceRepository) FindByInventoryIdAndResourceType(ctx context.Context, inventoryId *uuid.UUID, resourceType string) (*model.Resource, error) {
	args := r.Called(ctx, inventoryId, resourceType)
	return args.Get(0).(*model.Resource), args.Error(1)
}

func (r *MockedReporterResourceRepository) FindByReporterData(ctx context.Context, reporterId string, resourceId string) (*model.Resource, error) {
	args := r.Called(ctx, reporterId, resourceId)
	return args.Get(0).(*model.Resource), args.Error(1)
}

func (r *MockedReporterResourceRepository) ListAll(ctx context.Context) ([]*model.Resource, error) {
	args := r.Called(ctx)
	return args.Get(0).([]*model.Resource), args.Error(1)
}

func (r *MockedInventoryResourceRepository) FindByID(ctx context.Context, id uuid.UUID) (*model.InventoryResource, error) {
	args := r.Called(ctx, id)
	return args.Get(0).(*model.InventoryResource), args.Error(1)
}

func (r *MockedReporterResourceRepository) FindByWorkspaceId(ctx context.Context, workspace_id string) ([]*model.Resource, error) {
	args := r.Called(ctx)
	return args.Get(0).([]*model.Resource), args.Error(1)
}

func (m *MockedListenManager) Subscribe(txid string) pubsub.Subscription {
	args := m.Called(txid)
	return args.Get(0).(pubsub.Subscription)
}

func (m *MockedListenManager) WaitAndDistribute(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockedListenManager) Run(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockedSubscription) NotificationC() <-chan []byte {
	args := m.Called()
	return args.Get(0).(chan []byte)
}

func (m *MockedSubscription) Unsubscribe() {
	m.Called()
}

func (m *MockedSubscription) BlockForNotification(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}
