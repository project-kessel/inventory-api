package mocks

import (
	"context"

	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
	"github.com/go-kratos/kratos/v2/transport"
	pb "github.com/project-kessel/inventory-api/api/kessel/inventory/v1"
	"github.com/project-kessel/inventory-api/internal/biz/model"
	"github.com/project-kessel/inventory-api/internal/pubsub"

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

type MockRelationsRepository struct {
	mock.Mock
}

func (m *MockRelationsRepository) Health(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockRelationsRepository) Check(ctx context.Context, resource model.ReporterResourceKey, relation model.Relation,
	subject model.SubjectReference, consistency model.Consistency) (bool, model.ConsistencyToken, error) {
	args := m.Called(ctx, resource, relation, subject, consistency)
	return args.Bool(0), args.Get(1).(model.ConsistencyToken), args.Error(2)
}

func (m *MockRelationsRepository) CheckForUpdate(ctx context.Context, resource model.ReporterResourceKey, relation model.Relation,
	subject model.SubjectReference) (bool, model.ConsistencyToken, error) {
	args := m.Called(ctx, resource, relation, subject)
	return args.Bool(0), args.Get(1).(model.ConsistencyToken), args.Error(2)
}

func (m *MockRelationsRepository) CheckBulk(ctx context.Context, items []model.CheckItem,
	consistency model.Consistency) ([]model.CheckBulkResultItem, model.ConsistencyToken, error) {
	args := m.Called(ctx, items, consistency)
	return args.Get(0).([]model.CheckBulkResultItem), args.Get(1).(model.ConsistencyToken), args.Error(2)
}

func (m *MockRelationsRepository) LookupResources(ctx context.Context, query model.LookupResourcesQuery) (model.LookupResourcesIterator, error) {
	args := m.Called(ctx, query)
	return args.Get(0).(model.LookupResourcesIterator), args.Error(1)
}

func (m *MockRelationsRepository) CreateTuples(ctx context.Context, tuples []model.RelationsTuple, upsert bool,
	lockId, lockToken string) (model.ConsistencyToken, error) {
	args := m.Called(ctx, tuples, upsert, lockId, lockToken)
	return args.Get(0).(model.ConsistencyToken), args.Error(1)
}

func (m *MockRelationsRepository) DeleteTuples(ctx context.Context, tuples []model.RelationsTuple,
	lockId, lockToken string) (model.ConsistencyToken, error) {
	args := m.Called(ctx, tuples, lockId, lockToken)
	return args.Get(0).(model.ConsistencyToken), args.Error(1)
}

func (m *MockRelationsRepository) AcquireLock(ctx context.Context, lockId string) (string, error) {
	args := m.Called(ctx, lockId)
	return args.String(0), args.Error(1)
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
