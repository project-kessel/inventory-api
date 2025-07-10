package consumer

import (
	"context"
	"errors"
	"testing"

	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
	"github.com/project-kessel/inventory-api/internal/metricscollector"
	"github.com/stretchr/testify/mock"
	"go.opentelemetry.io/otel"

	"github.com/project-kessel/inventory-api/internal/biz/model_legacy"
	"github.com/project-kessel/inventory-api/internal/mocks"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/project-kessel/relations-api/api/kessel/relations/v1beta1"
	"github.com/stretchr/testify/assert"
	"gorm.io/gorm"

	. "github.com/project-kessel/inventory-api/cmd/common"
	"github.com/project-kessel/inventory-api/internal/authz"
	"github.com/project-kessel/inventory-api/internal/pubsub"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	testMessageKey            = `{"schema":{"type":"string","optional":false},"payload":"00000000-0000-0000-0000-000000000000"}`
	testCreateOrUpdateMessage = `{"schema":{"type":"string","optional":false,"name":"io.debezium.data.Json","version":1},"payload":{"subject":{"subject":{"id":"1234", "type":{"name":"workspace","namespace":"rbac"}}},"relation":"t_workspace","resource":{"id":"4321","type":{"name":"integration","namespace":"notifications"}}}}`
	testDeleteMessage         = `{"schema":{"type":"string","optional":false,"name":"io.debezium.data.Json","version":1},"payload":{"resource_id":"4321","resource_type":"integration","resource_namespace":"notifications","relation":"t_workspace","subject_filter":{"subject_type":"workspace","subject_namespace":"rbac","subject_id":"1234"}}}`
)

type TestCase struct {
	name            string
	description     string
	options         *Options
	config          *Config
	completedConfig CompletedConfig
	inv             InventoryConsumer
	metrics         metricscollector.MetricsCollector
	logger          *log.Helper
}

// TestSetup creates a test struct that calls most of the initial constructor methods we intend to test in unit tests.
func (t *TestCase) TestSetup() []error {
	t.options = NewOptions()
	t.options.BootstrapServers = []string{"localhost:9092"}
	t.config = NewConfig(t.options)
	t.config.AuthConfig.Enabled = false

	_, logger := InitLogger("info", LoggerOptions{})
	t.logger = log.NewHelper(log.With(logger, "subsystem", "inventoryConsumer"))

	var errs []error
	var err error

	if errList := t.options.Complete(); errList != nil {
		errs = append(errs, errList...)
	}
	if errList := t.options.Validate(); errList != nil {
		errs = append(errs, errList...)
	}
	cfg, errList := NewConfig(t.options).Complete()
	t.completedConfig = cfg
	if errList != nil {
		errs = append(errs, errList...)
	}

	notifier := &pubsub.NotifierMock{}

	authorizer := &mocks.MockAuthz{}
	createTupleResponse := &v1beta1.CreateTuplesResponse{ConsistencyToken: &v1beta1.ConsistencyToken{Token: "test-token"}}
	deleteTupleResponse := &v1beta1.DeleteTuplesResponse{ConsistencyToken: &v1beta1.ConsistencyToken{Token: "test-token"}}
	authorizer.On("CreateTuples", mock.Anything, mock.Anything).Return(createTupleResponse, nil)
	authorizer.On("DeleteTuples", mock.Anything, mock.Anything).Return(deleteTupleResponse, nil)

	consumer := mocks.MockConsumer{}
	t.inv, err = New(cfg, &gorm.DB{}, authz.CompletedConfig{}, authorizer, notifier, t.logger)
	if err != nil {
		errs = append(errs, err)
	}
	t.inv.Consumer = &consumer
	err = t.metrics.New(otel.Meter("github.com/project-kessel/inventory-api/blob/main/internal/server/otel"))
	if err != nil {
		errs = append(errs, err)
	}

	return errs
}

func makeTuple() *v1beta1.Relationship {
	return &v1beta1.Relationship{
		Resource: &v1beta1.ObjectReference{
			Type: &v1beta1.ObjectType{
				Namespace: "notifications",
				Name:      "integration",
			},
			Id: "4321",
		},
		Relation: "t_workspace",
		Subject: &v1beta1.SubjectReference{
			Subject: &v1beta1.ObjectReference{
				Type: &v1beta1.ObjectType{
					Namespace: "rbac",
					Name:      "workspace",
				},
				Id: "1234",
			},
		},
	}
}

func makeFilter() *v1beta1.RelationTupleFilter {
	return &v1beta1.RelationTupleFilter{
		ResourceNamespace: ToPointer("notifications"),
		ResourceType:      ToPointer("integration"),
		ResourceId:        ToPointer("4321"),
		Relation:          ToPointer("t_workspace"),
		SubjectFilter: &v1beta1.SubjectFilter{
			SubjectNamespace: ToPointer("rbac"),
			SubjectType:      ToPointer("workspace"),
			SubjectId:        ToPointer("1234"),
		},
	}
}

func TestNewConsumerSetup(t *testing.T) {
	test := TestCase{
		name:        "TestNewConsumerSetup",
		description: "ensures setting up a new consumer, including options and configs functions",
	}
	errs := test.TestSetup()
	assert.Nil(t, errs)
}

func TestParseCreateOrUpdateMessage(t *testing.T) {
	expected := makeTuple()
	tuple, err := ParseCreateOrUpdateMessage([]byte(testCreateOrUpdateMessage))
	assert.Nil(t, err)
	assert.Equal(t, tuple.Subject.Subject.Id, expected.Subject.Subject.Id)
	assert.Equal(t, tuple.Subject.Subject.Type.Name, expected.Subject.Subject.Type.Name)
	assert.Equal(t, tuple.Subject.Subject.Type.Namespace, expected.Subject.Subject.Type.Namespace)
	assert.Equal(t, tuple.Relation, expected.Relation)
	assert.Equal(t, tuple.Resource.Id, expected.Resource.Id)
	assert.Equal(t, tuple.Resource.Type.Name, expected.Resource.Type.Name)
	assert.Equal(t, tuple.Resource.Type.Namespace, expected.Resource.Type.Namespace)
}

func TestParseDeleteMessage(t *testing.T) {
	expected := makeFilter()
	filter, err := ParseDeleteMessage([]byte(testDeleteMessage))
	assert.Nil(t, err)
	assert.Equal(t, filter.ResourceId, expected.ResourceId)
	assert.Equal(t, filter.ResourceType, expected.ResourceType)
	assert.Equal(t, filter.ResourceNamespace, expected.ResourceNamespace)
	assert.Equal(t, filter.Relation, expected.Relation)
	assert.Equal(t, *filter.SubjectFilter.SubjectId, *expected.SubjectFilter.SubjectId)
	assert.Equal(t, *filter.SubjectFilter.SubjectType, *expected.SubjectFilter.SubjectType)
	assert.Equal(t, *filter.SubjectFilter.SubjectNamespace, *expected.SubjectFilter.SubjectNamespace)
}

func TestParseMessageKey(t *testing.T) {
	expected := "00000000-0000-0000-0000-000000000000"
	key, err := ParseMessageKey([]byte(testMessageKey))
	assert.Nil(t, err)
	assert.Equal(t, key, expected)
}

func TestParseHeaders(t *testing.T) {
	tests := []struct {
		name              string
		expectedOperation string
		expectedTxid      string
		msg               *kafka.Message
		expectErr         bool
	}{
		{
			name:              "Create Operation",
			expectedOperation: string(model_legacy.OperationTypeCreated),
			expectedTxid:      "123456",
			msg: &kafka.Message{
				Headers: []kafka.Header{
					{Key: "operation", Value: []byte(string(model_legacy.OperationTypeCreated))},
					{Key: "txid", Value: []byte("123456")},
				},
			},
			expectErr: false,
		},
		{
			name:              "Update Operation",
			expectedOperation: string(model_legacy.OperationTypeUpdated),
			expectedTxid:      "123456",
			msg: &kafka.Message{
				Headers: []kafka.Header{
					{Key: "operation", Value: []byte(string(model_legacy.OperationTypeUpdated))},
					{Key: "txid", Value: []byte("123456")},
				},
			},
			expectErr: false,
		},
		{
			name:              "Delete Operation",
			expectedOperation: string(model_legacy.OperationTypeDeleted),
			expectedTxid:      "",
			msg: &kafka.Message{
				Headers: []kafka.Header{
					{Key: "operation", Value: []byte(string(model_legacy.OperationTypeDeleted))},
					{Key: "txid", Value: []byte{}},
				},
			},
			expectErr: false,
		},
		{
			name:              "No Txid Value",
			expectedOperation: "any",
			expectedTxid:      "",
			msg: &kafka.Message{
				Headers: []kafka.Header{
					{Key: "operation", Value: []byte("any")},
					{Key: "txid", Value: []byte{}},
				},
			},
			expectErr: false,
		},
		{
			name:              "Missing Txid Header",
			expectedOperation: "any",
			expectedTxid:      "",
			msg: &kafka.Message{
				Headers: []kafka.Header{
					{Key: "operation", Value: []byte("any")},
				},
			},
			expectErr: true,
		},
		{
			name:              "Missing Operation Header",
			expectedOperation: "",
			expectedTxid:      "123456",
			msg: &kafka.Message{
				Headers: []kafka.Header{
					{Key: "txid", Value: []byte("123456")},
				},
			},
			expectErr: true,
		},
		{
			name:              "Missing Operation Value",
			expectedOperation: "",
			expectedTxid:      "123456",
			msg: &kafka.Message{
				Headers: []kafka.Header{
					{Key: "operation", Value: []byte{}},
					{Key: "txid", Value: []byte("123456")},
				},
			},
			expectErr: true,
		},
		{
			name:              "Extra Headers",
			expectedOperation: string(model_legacy.OperationTypeCreated),
			expectedTxid:      "123456",
			msg: &kafka.Message{
				Headers: []kafka.Header{
					{Key: "operation", Value: []byte(string(model_legacy.OperationTypeCreated))},
					{Key: "txid", Value: []byte("123456")},
					{Key: "unused-header", Value: []byte("unused-header-data")},
				},
			},
			expectErr: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			parsedHeaders, err := ParseHeaders(test.msg)
			if test.expectErr {
				assert.NotNil(t, err)
				assert.Nil(t, parsedHeaders)
			} else {
				assert.Nil(t, err)
				assert.Equal(t, parsedHeaders["operation"], test.expectedOperation)
				assert.Equal(t, parsedHeaders["txid"], test.expectedTxid)
				assert.LessOrEqual(t, len(parsedHeaders), 2)
			}
		})
	}
}

func TestInventoryConsumer_Retry(t *testing.T) {
	tests := []struct {
		description   string
		funcToExecute func() (string, error)
		exectedResult string
		expectedErr   error
	}{
		{
			description:   "retry returns no error after executing function",
			funcToExecute: func() (string, error) { return "success", nil },
			exectedResult: "success",
			expectedErr:   nil,
		},
		{
			description:   "retry fails and returns MaxRetriesError",
			funcToExecute: func() (string, error) { return "fail", ErrMaxRetries },
			exectedResult: "",
			expectedErr:   ErrMaxRetries,
		},
	}

	for _, test := range tests {
		t.Run(test.description, func(t *testing.T) {
			tester := TestCase{
				name:        "TestInventoryConsumer-Retry",
				description: test.description,
			}
			errs := tester.TestSetup()
			assert.Nil(t, errs)

			result, err := tester.inv.Retry(test.funcToExecute, tester.inv.MetricsCollector.MsgProcessFailures)
			assert.Equal(t, test.exectedResult, result)
			assert.Equal(t, test.expectedErr, err)
		})
	}
}

func TestInventoryConsumer_ProcessMessage(t *testing.T) {
	tests := []struct {
		name              string
		expectedOperation string
		expectedTxid      string
		msg               *kafka.Message
		relationsEnabled  bool
	}{
		{
			name:              "Create Operation",
			expectedOperation: string(model_legacy.OperationTypeCreated),
			expectedTxid:      "123456",
			msg: &kafka.Message{
				Key:   []byte(testMessageKey),
				Value: []byte(testCreateOrUpdateMessage),
			},
			relationsEnabled: true,
		},
		{
			name:              "Update Operation",
			expectedOperation: string(model_legacy.OperationTypeUpdated),
			expectedTxid:      "123456",
			msg: &kafka.Message{
				Key:   []byte(testMessageKey),
				Value: []byte(testCreateOrUpdateMessage),
			},
			relationsEnabled: true,
		},
		{
			name:              "Delete Operation",
			expectedOperation: string(model_legacy.OperationTypeDeleted),
			expectedTxid:      "",
			msg: &kafka.Message{
				Key:   []byte(testMessageKey),
				Value: []byte(testDeleteMessage),
			},
			relationsEnabled: true,
		},
		{
			name:              "Fake Operation",
			expectedOperation: "fake-operation",
			expectedTxid:      "123456",
			msg:               &kafka.Message{},
			relationsEnabled:  true,
		},
		{
			name:              "Created but relations disabled",
			expectedOperation: string(model_legacy.OperationTypeCreated),
			expectedTxid:      "123456",
			msg:               &kafka.Message{},
			relationsEnabled:  false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			tester := TestCase{}
			errs := tester.TestSetup()
			assert.Nil(t, errs)

			headers := []kafka.Header{
				{Key: "operation", Value: []byte(test.expectedOperation)},
				{Key: "txid", Value: []byte(test.expectedTxid)},
			}
			test.msg.Headers = headers
			parsedHeaders, err := ParseHeaders(test.msg)
			assert.Nil(t, err)
			assert.Equal(t, parsedHeaders["operation"], test.expectedOperation)
			assert.Equal(t, parsedHeaders["txid"], test.expectedTxid)

			if (test.expectedOperation == string(model_legacy.OperationTypeCreated) || test.expectedOperation == string(model_legacy.OperationTypeUpdated)) && test.relationsEnabled {
				resp, err := tester.inv.ProcessMessage(parsedHeaders, test.relationsEnabled, test.msg)
				assert.Nil(t, err)
				assert.Equal(t, "test-token", resp)
			} else {
				resp, err := tester.inv.ProcessMessage(parsedHeaders, test.relationsEnabled, test.msg)
				assert.Nil(t, err)
				assert.Equal(t, "", resp)
			}
		})
	}
}

func TestCheckIfCommit(t *testing.T) {
	tests := []struct {
		name      string
		partition kafka.TopicPartition
		expected  bool
	}{
		{
			name: "modulus of the partition offset equates to true",
			partition: kafka.TopicPartition{
				Offset: kafka.Offset(10),
			},
			expected: true,
		},
		{
			name: "modulus of the partition offset does not equate to true",
			partition: kafka.TopicPartition{
				Offset: kafka.Offset(1),
			},
			expected: false,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			actual := checkIfCommit(test.partition)
			assert.Equal(t, test.expected, actual)
		})
	}
}

func TestCommitStoredOffsets(t *testing.T) {
	tests := []struct {
		name                                      string
		storedOffsets, response, remainingOffsets []kafka.TopicPartition
		responseErr                               error
	}{
		{
			name: "single stored offset is committed without error",
			storedOffsets: []kafka.TopicPartition{
				{Offset: kafka.Offset(10), Partition: 0},
			},
			response: []kafka.TopicPartition{
				{Offset: kafka.Offset(10), Partition: 0},
			},
			remainingOffsets: nil,
			responseErr:      nil,
		},
		{
			name: "all stored offsets are committed without error",
			storedOffsets: []kafka.TopicPartition{
				{Offset: kafka.Offset(10), Partition: 0},
				{Offset: kafka.Offset(11), Partition: 0},
				{Offset: kafka.Offset(1), Partition: 1},
				{Offset: kafka.Offset(2), Partition: 1},
				{Offset: kafka.Offset(12), Partition: 0},
				{Offset: kafka.Offset(13), Partition: 0},
				{Offset: kafka.Offset(3), Partition: 1},
				{Offset: kafka.Offset(4), Partition: 1},
			},
			response: []kafka.TopicPartition{
				{Offset: kafka.Offset(10), Partition: 0},
				{Offset: kafka.Offset(11), Partition: 0},
				{Offset: kafka.Offset(1), Partition: 1},
				{Offset: kafka.Offset(2), Partition: 1},
				{Offset: kafka.Offset(12), Partition: 0},
				{Offset: kafka.Offset(13), Partition: 0},
				{Offset: kafka.Offset(3), Partition: 1},
				{Offset: kafka.Offset(4), Partition: 1},
			},
			remainingOffsets: nil,
			responseErr:      nil,
		},
		{
			name: "Consumer.CommitOffsets returns error; offset storage is not cleared",
			storedOffsets: []kafka.TopicPartition{
				{Offset: kafka.Offset(10), Partition: 1},
			},
			response:         nil,
			remainingOffsets: []kafka.TopicPartition{{Offset: kafka.Offset(10), Partition: 1}},
			responseErr:      errors.New("commit failed"),
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			tester := TestCase{}
			errs := tester.TestSetup()
			assert.Nil(t, errs)

			c := &mocks.MockConsumer{}
			c.On("CommitOffsets", mock.Anything).Return(test.response, test.responseErr)
			tester.inv.Consumer = c
			tester.inv.OffsetStorage = test.storedOffsets

			err := tester.inv.commitStoredOffsets()
			assert.Equal(t, err, test.responseErr)
			assert.Equal(t, len(tester.inv.OffsetStorage), len(test.remainingOffsets))
			assert.Equal(t, tester.inv.OffsetStorage, test.remainingOffsets)
		})
	}
}

func TestRebalanceCallback_AssignedPartitions(t *testing.T) {
	tests := []struct {
		name                  string
		partitions            []kafka.TopicPartition
		lockResponse          *v1beta1.AcquireLockResponse
		lockError             error
		expectedLockId        string
		expectedLockToken     string
		expectedError         error
		expectAcquireLockCall bool
	}{
		{
			name: "successful lock acquisition",
			partitions: []kafka.TopicPartition{
				{Partition: 0, Topic: ToPointer("test-topic")},
			},
			lockResponse: &v1beta1.AcquireLockResponse{
				LockToken: "test-lock-token-123",
			},
			lockError:             nil,
			expectedLockId:        "inventory-consumer/0",
			expectedLockToken:     "test-lock-token-123",
			expectedError:         nil,
			expectAcquireLockCall: true,
		},
		{
			name: "lock acquisition fails",
			partitions: []kafka.TopicPartition{
				{Partition: 0, Topic: ToPointer("test-topic")},
			},
			lockResponse:          nil,
			lockError:             errors.New("lock acquisition failed"),
			expectedLockId:        "inventory-consumer/0",
			expectedLockToken:     "",
			expectedError:         ErrMaxRetries,
			expectAcquireLockCall: true,
		},
		{
			name:                  "no partitions assigned",
			partitions:            []kafka.TopicPartition{},
			lockResponse:          nil,
			lockError:             nil,
			expectedLockId:        "",
			expectedLockToken:     "",
			expectedError:         nil,
			expectAcquireLockCall: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			tester := TestCase{}
			errs := tester.TestSetup()
			assert.Nil(t, errs)

			authorizer := &mocks.MockAuthz{}
			if test.expectAcquireLockCall {
				authorizer.On("AcquireLock", mock.Anything, mock.MatchedBy(func(req *v1beta1.AcquireLockRequest) bool {
					return req.LockId == test.expectedLockId
				})).Return(test.lockResponse, test.lockError)
			}
			tester.inv.Authorizer = authorizer

			event := kafka.AssignedPartitions{
				Partitions: test.partitions,
			}

			err := tester.inv.RebalanceCallback(nil, event)

			assert.Equal(t, test.expectedError, err)
			assert.Equal(t, test.expectedLockId, tester.inv.lockId)
			assert.Equal(t, test.expectedLockToken, tester.inv.lockToken)

			if test.expectAcquireLockCall {
				authorizer.AssertExpectations(t)
			}
		})
	}
}

func TestRebalanceCallback_RevokedPartitions(t *testing.T) {
	tests := []struct {
		name              string
		partitions        []kafka.TopicPartition
		storedOffsets     []kafka.TopicPartition
		commitResponse    []kafka.TopicPartition
		commitError       error
		assignmentLost    bool
		expectedError     error
		initialLockId     string
		initialLockToken  string
		expectedLockId    string
		expectedLockToken string
	}{
		{
			name: "successful partition revocation with offset commit",
			partitions: []kafka.TopicPartition{
				{Partition: 0, Topic: ToPointer("test-topic")},
			},
			storedOffsets: []kafka.TopicPartition{
				{Offset: kafka.Offset(10), Partition: 0},
			},
			commitResponse: []kafka.TopicPartition{
				{Offset: kafka.Offset(10), Partition: 0},
			},
			commitError:       nil,
			assignmentLost:    false,
			expectedError:     nil,
			initialLockId:     "inventory-consumer/0",
			initialLockToken:  "test-lock-token-123",
			expectedLockId:    "",
			expectedLockToken: "",
		},
		{
			name: "partition revocation with assignment lost",
			partitions: []kafka.TopicPartition{
				{Partition: 0, Topic: ToPointer("test-topic")},
			},
			storedOffsets: []kafka.TopicPartition{
				{Offset: kafka.Offset(5), Partition: 0},
			},
			commitResponse: []kafka.TopicPartition{
				{Offset: kafka.Offset(5), Partition: 0},
			},
			commitError:       nil,
			assignmentLost:    true,
			expectedError:     nil,
			initialLockId:     "inventory-consumer/0",
			initialLockToken:  "test-lock-token-456",
			expectedLockId:    "",
			expectedLockToken: "",
		},
		{
			name: "partition revocation with commit failure",
			partitions: []kafka.TopicPartition{
				{Partition: 1, Topic: ToPointer("test-topic")},
			},
			storedOffsets: []kafka.TopicPartition{
				{Offset: kafka.Offset(20), Partition: 1},
			},
			commitResponse:    nil,
			commitError:       errors.New("commit failed"),
			assignmentLost:    false,
			expectedError:     errors.New("commit failed"),
			initialLockId:     "inventory-consumer/1",
			initialLockToken:  "test-lock-token-789",
			expectedLockId:    "",
			expectedLockToken: "",
		},
		{
			name:              "partition revocation with no stored offsets",
			partitions:        []kafka.TopicPartition{{Partition: 0, Topic: ToPointer("test-topic")}},
			storedOffsets:     []kafka.TopicPartition{},
			commitResponse:    []kafka.TopicPartition{},
			commitError:       nil,
			assignmentLost:    false,
			expectedError:     nil,
			initialLockId:     "inventory-consumer/0",
			initialLockToken:  "test-lock-token-empty",
			expectedLockId:    "",
			expectedLockToken: "",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			tester := TestCase{}
			errs := tester.TestSetup()
			assert.Nil(t, errs)

			consumer := &mocks.MockConsumer{}
			consumer.On("AssignmentLost").Return(test.assignmentLost)
			consumer.On("CommitOffsets", test.storedOffsets).Return(test.commitResponse, test.commitError)
			tester.inv.Consumer = consumer

			tester.inv.OffsetStorage = test.storedOffsets
			tester.inv.lockId = test.initialLockId
			tester.inv.lockToken = test.initialLockToken

			event := kafka.RevokedPartitions{
				Partitions: test.partitions,
			}

			err := tester.inv.RebalanceCallback(nil, event)

			assert.Equal(t, test.expectedError, err)
			assert.Equal(t, test.expectedLockId, tester.inv.lockId)
			assert.Equal(t, test.expectedLockToken, tester.inv.lockToken)

			consumer.AssertExpectations(t)
		})
	}
}

func TestFencingToken_WritingAfterRebalance(t *testing.T) {
	// Setup two consumers
	testerA := TestCase{}
	errsA := testerA.TestSetup()
	assert.Nil(t, errsA)
	testerA.inv.Config.ConsumerGroupID = "test-group"

	testerB := TestCase{}
	errsB := testerB.TestSetup()
	assert.Nil(t, errsB)
	testerB.inv.Config.ConsumerGroupID = "test-group"

	authorizer := &mocks.MockAuthz{}
	createSuccessResponse := &v1beta1.CreateTuplesResponse{ConsistencyToken: &v1beta1.ConsistencyToken{Token: "test-token"}}

	consumerAToken := "token-A"
	consumerBToken := "token-B"
	lockCall1 := authorizer.On("AcquireLock", mock.Anything, mock.MatchedBy(func(req *v1beta1.AcquireLockRequest) bool {
		return req.LockId == "test-group/0"
	})).Return(&v1beta1.AcquireLockResponse{LockToken: consumerAToken}, nil).Once()

	lockCall2 := authorizer.On("AcquireLock", mock.Anything, mock.MatchedBy(func(req *v1beta1.AcquireLockRequest) bool {
		return req.LockId == "test-group/0"
	})).Return(&v1beta1.AcquireLockResponse{LockToken: consumerBToken}, nil).Once()

	// Ensure the calls happen in the right order
	lockCall2.NotBefore(lockCall1)

	// Mock the CreateTuples calls to only accept consumer B's token
	authorizer.On("CreateTuples", mock.Anything, mock.MatchedBy(func(req *v1beta1.CreateTuplesRequest) bool {
		return req.FencingCheck != nil && req.FencingCheck.LockToken == consumerBToken
	})).Return(createSuccessResponse, nil)

	authorizer.On("CreateTuples", mock.Anything, mock.MatchedBy(func(req *v1beta1.CreateTuplesRequest) bool {
		return req.FencingCheck != nil && req.FencingCheck.LockToken == consumerAToken
	})).Return((*v1beta1.CreateTuplesResponse)(nil), errors.New("fencing token is invalid or expired"))

	testerA.inv.Authorizer = authorizer
	testerB.inv.Authorizer = authorizer

	assignedPartitions := kafka.AssignedPartitions{
		Partitions: []kafka.TopicPartition{{Partition: 0, Topic: ToPointer("test-topic")}},
	}

	// Consumer A acquires the lock first
	err := testerA.inv.RebalanceCallback(nil, assignedPartitions)
	assert.Nil(t, err)
	assert.Equal(t, "test-group/0", testerA.inv.lockId)
	assert.Equal(t, "token-A", testerA.inv.lockToken)
	staleToken := testerA.inv.lockToken

	// Consumer B acquires the lock, invalidating A's token
	err = testerB.inv.RebalanceCallback(nil, assignedPartitions)
	assert.Nil(t, err)
	assert.Equal(t, "test-group/0", testerB.inv.lockId)
	assert.Equal(t, "token-B", testerB.inv.lockToken)
	assert.NotEqual(t, staleToken, testerB.inv.lockToken)

	// Simulate consumer A waking up with the stale token
	testerA.inv.lockToken = staleToken

	// Try to create a tuple with the stale token
	tuple := makeTuple()
	_, err = testerA.inv.CreateTuple(context.Background(), tuple)
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "fencing token is invalid or expired")

	// Consumer B should be able to create a tuple with the valid token
	resp, err := testerB.inv.CreateTuple(context.Background(), tuple)
	assert.Nil(t, err)
	assert.Equal(t, "test-token", resp)

	authorizer.AssertExpectations(t)
}

func TestInventoryConsumer_CreateTuple_FailedPrecondition(t *testing.T) {
	tester := TestCase{}
	errs := tester.TestSetup()
	assert.Nil(t, errs)

	authorizer := &mocks.MockAuthz{}
	authorizer.On("CreateTuples", mock.Anything, mock.Anything).Return((*v1beta1.CreateTuplesResponse)(nil), status.Error(codes.FailedPrecondition, "invalid fencing token"))

	tester.inv.Authorizer = authorizer
	tester.inv.lockToken = "test-token"

	tuple := makeTuple()
	resp, err := tester.inv.CreateTuple(context.Background(), tuple)

	assert.NotNil(t, err)
	assert.Equal(t, "", resp)
	assert.Contains(t, err.Error(), "invalid fencing token")

	authorizer.AssertExpectations(t)
}

func TestInventoryConsumer_UpdateTuple_FailedPrecondition(t *testing.T) {
	tester := TestCase{}
	errs := tester.TestSetup()
	assert.Nil(t, errs)

	authorizer := &mocks.MockAuthz{}
	authorizer.On("CreateTuples", mock.Anything, mock.Anything).Return((*v1beta1.CreateTuplesResponse)(nil), status.Error(codes.FailedPrecondition, "invalid fencing token"))

	tester.inv.Authorizer = authorizer
	tester.inv.lockToken = "test-token"

	tuple := makeTuple()
	resp, err := tester.inv.UpdateTuple(context.Background(), tuple)

	assert.NotNil(t, err)
	assert.Equal(t, "", resp)
	assert.Contains(t, err.Error(), "invalid fencing token")

	authorizer.AssertExpectations(t)
}

func TestInventoryConsumer_DeleteTuple_FailedPrecondition(t *testing.T) {
	tester := TestCase{}
	errs := tester.TestSetup()
	assert.Nil(t, errs)

	authorizer := &mocks.MockAuthz{}
	authorizer.On("DeleteTuples", mock.Anything, mock.Anything).Return((*v1beta1.DeleteTuplesResponse)(nil), status.Error(codes.FailedPrecondition, "invalid fencing token"))

	tester.inv.Authorizer = authorizer
	tester.inv.lockToken = "test-token"

	filter := makeFilter()
	resp, err := tester.inv.DeleteTuple(context.Background(), filter)

	assert.NotNil(t, err)
	assert.Equal(t, "", resp)
	assert.Contains(t, err.Error(), "invalid fencing token")

	authorizer.AssertExpectations(t)
}
