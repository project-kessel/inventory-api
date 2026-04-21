package mocks

import (
	"context"
	"io"

	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
	"github.com/go-kratos/kratos/v2/transport"
	pb "github.com/project-kessel/inventory-api/api/kessel/inventory/v1"
	"github.com/project-kessel/inventory-api/internal/pubsub"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	kesselv1 "github.com/project-kessel/relations-api/api/kessel/relations/v1"
	"github.com/project-kessel/relations-api/api/kessel/relations/v1beta1"
	"github.com/stretchr/testify/mock"
)

type MockHealthRepo struct {
	mock.Mock
}

func (m *MockHealthRepo) IsBackendAvailable(ctx context.Context) (*pb.GetReadyzResponse, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*pb.GetReadyzResponse), args.Error(1)
}

func (m *MockHealthRepo) IsRelationsAvailable(ctx context.Context) (*pb.GetReadyzResponse, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*pb.GetReadyzResponse), args.Error(1)

}

// MockRelationsRepository is a minimal mock for RelationsRepository.
// It only implements methods needed for edge-case tests that SimpleRelationsRepository cannot produce:
//   - CheckBulk: for response length mismatches
//   - CheckForUpdateBulk: for per-pair errors
//   - AcquireLock: for complex fencing token validation scenarios
//   - CreateTuples: for complex fencing token validation scenarios
//
// All other methods panic to enforce use of SimpleRelationsRepository (a fake) for normal test cases.
type MockRelationsRepository struct {
	mock.Mock
}

func (m *MockRelationsRepository) CheckBulk(ctx context.Context, req *v1beta1.CheckBulkRequest) (*v1beta1.CheckBulkResponse, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*v1beta1.CheckBulkResponse), args.Error(1)
}

func (m *MockRelationsRepository) CheckForUpdateBulk(ctx context.Context, req *v1beta1.CheckForUpdateBulkRequest) (*v1beta1.CheckForUpdateBulkResponse, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*v1beta1.CheckForUpdateBulkResponse), args.Error(1)
}

func (m *MockRelationsRepository) AcquireLock(ctx context.Context, req *v1beta1.AcquireLockRequest) (*v1beta1.AcquireLockResponse, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*v1beta1.AcquireLockResponse), args.Error(1)
}

func (m *MockRelationsRepository) CreateTuples(ctx context.Context, req *v1beta1.CreateTuplesRequest) (*v1beta1.CreateTuplesResponse, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*v1beta1.CreateTuplesResponse), args.Error(1)
}

// Stub methods that panic to enforce use of SimpleRelationsRepository for normal test cases.
// MockRelationsRepository should only be used for edge cases that cannot be produced by SimpleRelationsRepository.

func (m *MockRelationsRepository) Health(ctx context.Context) (*kesselv1.GetReadyzResponse, error) {
	panic("MockRelationsRepository.Health() is not supported - use SimpleRelationsRepository instead")
}

func (m *MockRelationsRepository) ReadTuples(ctx context.Context, request *v1beta1.ReadTuplesRequest) (grpc.ServerStreamingClient[v1beta1.ReadTuplesResponse], error) {
	panic("MockRelationsRepository.ReadTuples() is not supported - use SimpleRelationsRepository instead")
}

func (m *MockRelationsRepository) Check(ctx context.Context, namespace string, permission string, consistencyToken string, resourceType string, localResourceId string, sub *v1beta1.SubjectReference) (v1beta1.CheckResponse_Allowed, *v1beta1.ConsistencyToken, error) {
	panic("MockRelationsRepository.Check() is not supported - use SimpleRelationsRepository instead")
}

func (m *MockRelationsRepository) CheckForUpdate(ctx context.Context, namespace string, permission string, resourceType string, localResourceId string, sub *v1beta1.SubjectReference) (v1beta1.CheckForUpdateResponse_Allowed, *v1beta1.ConsistencyToken, error) {
	panic("MockRelationsRepository.CheckForUpdate() is not supported - use SimpleRelationsRepository instead")
}

func (m *MockRelationsRepository) LookupResources(ctx context.Context, in *v1beta1.LookupResourcesRequest) (grpc.ServerStreamingClient[v1beta1.LookupResourcesResponse], error) {
	panic("MockRelationsRepository.LookupResources() is not supported - use SimpleRelationsRepository instead")
}

func (m *MockRelationsRepository) LookupSubjects(ctx context.Context, in *v1beta1.LookupSubjectsRequest) (grpc.ServerStreamingClient[v1beta1.LookupSubjectsResponse], error) {
	panic("MockRelationsRepository.LookupSubjects() is not supported - use SimpleRelationsRepository instead")
}

func (m *MockRelationsRepository) DeleteTuples(ctx context.Context, req *v1beta1.DeleteTuplesRequest) (*v1beta1.DeleteTuplesResponse, error) {
	panic("MockRelationsRepository.DeleteTuples() is not supported - use SimpleRelationsRepository instead")
}

func (m *MockRelationsRepository) UnsetWorkspace(ctx context.Context, namespace string, resourceType string, localResourceId string) (*v1beta1.DeleteTuplesResponse, error) {
	panic("MockRelationsRepository.UnsetWorkspace() is not supported - use SimpleRelationsRepository instead")
}

func (m *MockRelationsRepository) SetWorkspace(ctx context.Context, namespace string, resourceType string, localResourceId string, workspaceId string, replace bool) (*v1beta1.CreateTuplesResponse, error) {
	panic("MockRelationsRepository.SetWorkspace() is not supported - use SimpleRelationsRepository instead")
}

type MockConsumer struct {
	mock.Mock
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

// MockTransporter is a test helper that implements transport.Transporter
type MockTransporter struct {
	OperationValue string
}

func (m *MockTransporter) Kind() transport.Kind            { return transport.KindHTTP }
func (m *MockTransporter) Endpoint() string                { return "/test" }
func (m *MockTransporter) Operation() string               { return m.OperationValue }
func (m *MockTransporter) RequestHeader() transport.Header { return &MockHeader{} }
func (m *MockTransporter) ReplyHeader() transport.Header   { return &MockHeader{} }

// MockHeader is a test helper that implements transport.Header
type MockHeader struct{}

func (m *MockHeader) Get(key string) string      { return "" }
func (m *MockHeader) Set(key, value string)      {}
func (m *MockHeader) Add(key, value string)      {}
func (m *MockHeader) Keys() []string             { return nil }
func (m *MockHeader) Values(key string) []string { return nil }
