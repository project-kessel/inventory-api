package mocks

import (
	"context"
	"io"

	"github.com/google/uuid"
	"github.com/project-kessel/inventory-api/internal/pubsub"
	"google.golang.org/grpc/metadata"

	pb "github.com/project-kessel/inventory-api/api/kessel/inventory/v1"

	"github.com/confluentinc/confluent-kafka-go/v2/kafka"

	authzapi "github.com/project-kessel/inventory-api/internal/authz/api"
	kessel "github.com/project-kessel/inventory-api/internal/authz/model"
	"github.com/stretchr/testify/mock"
	"google.golang.org/grpc"

	"github.com/project-kessel/inventory-api/internal/biz/model_legacy"
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
	Responses []*authzapi.ResourceResult
	current   int
}

// MockAuthz Authorizer interface implementation

func (m *MockAuthz) Health(ctx context.Context) (*kessel.GetReadyzResponse, error) {
	args := m.Called(ctx)
	return args.Get(0).(*kessel.GetReadyzResponse), args.Error(1)
}

func (m *MockAuthz) IsBackendAvailable() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockAuthz) Check(ctx context.Context, request *kessel.CheckRequest) (*kessel.CheckResponse, error) {
	args := m.Called(ctx, request)
	return args.Get(0).(*kessel.CheckResponse), args.Error(1)
}

func (m *MockAuthz) CheckForUpdate(ctx context.Context, request *kessel.CheckForUpdateRequest) (*kessel.CheckForUpdateResponse, error) {
	args := m.Called(ctx, request)
	return args.Get(0).(*kessel.CheckForUpdateResponse), args.Error(1)
}

func (m *MockAuthz) CheckBulk(ctx context.Context, req *kessel.CheckBulkRequest) (*kessel.CheckBulkResponse, error) {
	args := m.Called(ctx, req)
	return args.Get(0).(*kessel.CheckBulkResponse), args.Error(1)
}

func (m *MockAuthz) CreateRelationships(ctx context.Context, rels []*kessel.Relationship, touch authzapi.TouchSemantics, fencing *kessel.FencingCheck) (*kessel.CreateRelationshipsResponse, error) {
	args := m.Called(ctx, rels, touch, fencing)
	return args.Get(0).(*kessel.CreateRelationshipsResponse), args.Error(1)
}

func (m *MockAuthz) DeleteRelationships(ctx context.Context, filter *kessel.RelationTupleFilter, fencing *kessel.FencingCheck) (*kessel.DeleteRelationshipsResponse, error) {
	args := m.Called(ctx, filter, fencing)
	return args.Get(0).(*kessel.DeleteRelationshipsResponse), args.Error(1)
}

func (m *MockAuthz) ReadRelationships(ctx context.Context, filter *kessel.RelationTupleFilter, limit uint32, continuation authzapi.ContinuationToken, consistency *kessel.Consistency) (chan *authzapi.RelationshipResult, chan error, error) {
	args := m.Called(ctx, filter, limit, continuation, consistency)
	return args.Get(0).(chan *authzapi.RelationshipResult), args.Get(1).(chan error), args.Error(2)
}

func (m *MockAuthz) LookupResources(ctx context.Context, resourceType *kessel.ObjectType, relation string, subject *kessel.SubjectReference, limit uint32, continuation authzapi.ContinuationToken, consistency *kessel.Consistency) (chan *authzapi.ResourceResult, chan error, error) {
	args := m.Called(ctx, resourceType, relation, subject, limit, continuation, consistency)
	return args.Get(0).(chan *authzapi.ResourceResult), args.Get(1).(chan error), args.Error(2)
}

func (m *MockAuthz) LookupSubjects(ctx context.Context, subjectType *kessel.ObjectType, subjectRelation, relation string, resource *kessel.ObjectReference, limit uint32, continuation authzapi.ContinuationToken, consistency *kessel.Consistency) (chan *authzapi.SubjectResult, chan error, error) {
	args := m.Called(ctx, subjectType, subjectRelation, relation, resource, limit, continuation, consistency)
	return args.Get(0).(chan *authzapi.SubjectResult), args.Get(1).(chan error), args.Error(2)
}

func (m *MockAuthz) ImportBulkTuples(stream grpc.ClientStreamingServer[kessel.ImportBulkTuplesRequest, kessel.ImportBulkTuplesResponse]) error {
	args := m.Called(stream)
	return args.Error(0)
}

func (m *MockAuthz) AcquireLock(ctx context.Context, lockId string) (*kessel.AcquireLockResponse, error) {
	args := m.Called(ctx, lockId)
	return args.Get(0).(*kessel.AcquireLockResponse), args.Error(1)
}

// MockHealthRepo methods

func (m *MockHealthRepo) IsBackendAvailable(ctx context.Context) (*pb.GetReadyzResponse, error) {
	args := m.Called(ctx)
	return args.Get(0).(*pb.GetReadyzResponse), args.Error(1)
}

func (m *MockHealthRepo) IsRelationsRepositoryAvailable(ctx context.Context) (*pb.GetReadyzResponse, error) {
	args := m.Called(ctx)
	return args.Get(0).(*pb.GetReadyzResponse), args.Error(1)
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

func (m *MockLookupResourcesStream) Recv() (*authzapi.ResourceResult, error) {
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

func (r *MockedReporterResourceRepository) Create(ctx context.Context, resource *model_legacy.Resource, namespace string, txid string) (*model_legacy.Resource, error) {
	args := r.Called(ctx, resource, namespace, txid)
	return args.Get(0).(*model_legacy.Resource), args.Error(1)
}

func (r *MockedReporterResourceRepository) Update(ctx context.Context, resource *model_legacy.Resource, id uuid.UUID, namespace string, txid string) (*model_legacy.Resource, error) {
	args := r.Called(ctx, resource, id, namespace, txid)
	return args.Get(0).(*model_legacy.Resource), args.Error(1)
}

func (r *MockedReporterResourceRepository) Delete(ctx context.Context, id uuid.UUID, namespace string) (*model_legacy.Resource, error) {
	args := r.Called(ctx, id, namespace)
	return args.Get(0).(*model_legacy.Resource), args.Error(1)
}

func (r *MockedReporterResourceRepository) FindByID(ctx context.Context, id uuid.UUID) (*model_legacy.Resource, error) {
	args := r.Called(ctx, id)
	return args.Get(0).(*model_legacy.Resource), args.Error(1)
}

func (r *MockedReporterResourceRepository) FindByReporterResourceId(ctx context.Context, id model_legacy.ReporterResourceId) (*model_legacy.Resource, error) {
	args := r.Called(ctx, id)
	return args.Get(0).(*model_legacy.Resource), args.Error(1)
}

func (r *MockedReporterResourceRepository) FindByInventoryIdAndReporter(ctx context.Context, inventoryId *uuid.UUID, reporterResourceId string, reporterType string) (*model_legacy.Resource, error) {
	args := r.Called(ctx, inventoryId, reporterResourceId, reporterType)
	return args.Get(0).(*model_legacy.Resource), args.Error(1)
}

func (r *MockedReporterResourceRepository) FindByReporterResourceIdv1beta2(ctx context.Context, id model_legacy.ReporterResourceUniqueIndex) (*model_legacy.Resource, error) {
	args := r.Called(ctx, id)
	return args.Get(0).(*model_legacy.Resource), args.Error(1)
}

func (r *MockedReporterResourceRepository) FindByInventoryIdAndResourceType(ctx context.Context, inventoryId *uuid.UUID, resourceType string) (*model_legacy.Resource, error) {
	args := r.Called(ctx, inventoryId, resourceType)
	return args.Get(0).(*model_legacy.Resource), args.Error(1)
}

func (r *MockedReporterResourceRepository) FindByReporterData(ctx context.Context, reporterId string, resourceId string) (*model_legacy.Resource, error) {
	args := r.Called(ctx, reporterId, resourceId)
	return args.Get(0).(*model_legacy.Resource), args.Error(1)
}

func (r *MockedReporterResourceRepository) ListAll(ctx context.Context) ([]*model_legacy.Resource, error) {
	args := r.Called(ctx)
	return args.Get(0).([]*model_legacy.Resource), args.Error(1)
}

func (r *MockedInventoryResourceRepository) FindByID(ctx context.Context, id uuid.UUID) (*model_legacy.InventoryResource, error) {
	args := r.Called(ctx, id)
	return args.Get(0).(*model_legacy.InventoryResource), args.Error(1)
}

func (r *MockedReporterResourceRepository) FindByWorkspaceId(ctx context.Context, workspace_id string) ([]*model_legacy.Resource, error) {
	args := r.Called(ctx)
	return args.Get(0).([]*model_legacy.Resource), args.Error(1)
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
