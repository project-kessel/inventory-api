package mocks

import (
	"context"
	"io"

	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
	"github.com/go-kratos/kratos/v2/transport"
	"github.com/project-kessel/inventory-api/internal/biz/model"
	"github.com/project-kessel/inventory-api/internal/pubsub"

	"github.com/stretchr/testify/mock"
)

type MockHealthRepo struct {
	mock.Mock
}

func (m *MockHealthRepo) IsBackendAvailable(ctx context.Context) (model.HealthResult, error) {
	args := m.Called(ctx)
	return args.Get(0).(model.HealthResult), args.Error(1)
}

func (m *MockHealthRepo) IsRelationsAvailable(ctx context.Context) (model.HealthResult, error) {
	args := m.Called(ctx)
	return args.Get(0).(model.HealthResult), args.Error(1)
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

func (m *MockRelationsRepository) CheckBulk(ctx context.Context, rels []model.Relationship, consistency model.Consistency) (model.CheckBulkResult, error) {
	args := m.Called(ctx, rels, consistency)
	return args.Get(0).(model.CheckBulkResult), args.Error(1)
}

func (m *MockRelationsRepository) CheckForUpdateBulk(ctx context.Context, rels []model.Relationship) (model.CheckBulkResult, error) {
	args := m.Called(ctx, rels)
	return args.Get(0).(model.CheckBulkResult), args.Error(1)
}

func (m *MockRelationsRepository) AcquireLock(ctx context.Context, lockId model.LockId) (model.AcquireLockResult, error) {
	args := m.Called(ctx, lockId)
	return args.Get(0).(model.AcquireLockResult), args.Error(1)
}

func (m *MockRelationsRepository) CreateTuples(ctx context.Context, tuples []model.RelationsTuple, upsert bool, fencing *model.FencingCheck) (model.TuplesResult, error) {
	args := m.Called(ctx, tuples, upsert, fencing)
	return args.Get(0).(model.TuplesResult), args.Error(1)
}

func (m *MockRelationsRepository) Check(ctx context.Context, rel model.Relationship, consistency model.Consistency) (model.CheckResult, error) {
	args := m.Called(ctx, rel, consistency)
	return args.Get(0).(model.CheckResult), args.Error(1)
}

func (m *MockRelationsRepository) Health(_ context.Context) (model.HealthResult, error) {
	panic("MockRelationsRepository.Health() is not supported - use SimpleRelationsRepository instead")
}

func (m *MockRelationsRepository) ReadTuples(_ context.Context, _ model.TupleFilter, _ *model.Pagination, _ model.Consistency) (model.ResultStream[model.ReadTuplesItem], error) {
	panic("MockRelationsRepository.ReadTuples() is not supported - use SimpleRelationsRepository instead")
}

func (m *MockRelationsRepository) CheckForUpdate(_ context.Context, _ model.Relationship) (model.CheckResult, error) {
	panic("MockRelationsRepository.CheckForUpdate() is not supported - use SimpleRelationsRepository instead")
}

func (m *MockRelationsRepository) LookupObjects(_ context.Context, _ model.RepresentationType, _ model.Relation, _ model.SubjectReference, _ *model.Pagination, _ model.Consistency) (model.ResultStream[model.LookupObjectsItem], error) {
	panic("MockRelationsRepository.LookupObjects() is not supported - use SimpleRelationsRepository instead")
}

func (m *MockRelationsRepository) LookupSubjects(_ context.Context, _ model.ResourceReference, _ model.Relation, _ model.RepresentationType, _ *model.Relation, _ *model.Pagination, _ model.Consistency) (model.ResultStream[model.LookupSubjectsItem], error) {
	panic("MockRelationsRepository.LookupSubjects() is not supported - use SimpleRelationsRepository instead")
}

func (m *MockRelationsRepository) DeleteTuples(_ context.Context, _ model.TupleFilter, _ *model.FencingCheck) (model.TuplesResult, error) {
	panic("MockRelationsRepository.DeleteTuples() is not supported - use SimpleRelationsRepository instead")
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

type MockLookupObjectsStream struct {
	Responses []model.LookupObjectsItem
	current   int
}

func (m *MockLookupObjectsStream) Recv() (model.LookupObjectsItem, error) {
	if m.current >= len(m.Responses) {
		return model.LookupObjectsItem{}, io.EOF
	}
	res := m.Responses[m.current]
	m.current++
	return res, nil
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
