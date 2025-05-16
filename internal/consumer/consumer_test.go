package consumer

import (
	"errors"
	"testing"

	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
	"github.com/project-kessel/inventory-api/internal/metricscollector"
	"github.com/stretchr/testify/mock"

	"github.com/project-kessel/inventory-api/internal/biz/model"
	"github.com/project-kessel/inventory-api/internal/mocks"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/project-kessel/relations-api/api/kessel/relations/v1beta1"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/otel"
	"gorm.io/gorm"

	. "github.com/project-kessel/inventory-api/cmd/common"
	"github.com/project-kessel/inventory-api/internal/authz"
	"github.com/project-kessel/inventory-api/internal/pubsub"
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

	t.inv, err = New(t.completedConfig, &gorm.DB{}, authz.CompletedConfig{}, authorizer, notifier, t.logger)
	if err != nil {
		errs = append(errs, err)
	}

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
			expectedOperation: string(model.OperationTypeCreated),
			expectedTxid:      "123456",
			msg: &kafka.Message{
				Headers: []kafka.Header{
					{Key: "operation", Value: []byte(string(model.OperationTypeCreated))},
					{Key: "txid", Value: []byte("123456")},
				},
			},
			expectErr: false,
		},
		{
			name:              "Update Operation",
			expectedOperation: string(model.OperationTypeUpdated),
			expectedTxid:      "123456",
			msg: &kafka.Message{
				Headers: []kafka.Header{
					{Key: "operation", Value: []byte(string(model.OperationTypeUpdated))},
					{Key: "txid", Value: []byte("123456")},
				},
			},
			expectErr: false,
		},
		{
			name:              "Delete Operation",
			expectedOperation: string(model.OperationTypeDeleted),
			expectedTxid:      "",
			msg: &kafka.Message{
				Headers: []kafka.Header{
					{Key: "operation", Value: []byte(string(model.OperationTypeDeleted))},
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
			expectedOperation: string(model.OperationTypeCreated),
			expectedTxid:      "123456",
			msg: &kafka.Message{
				Headers: []kafka.Header{
					{Key: "operation", Value: []byte(string(model.OperationTypeCreated))},
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

			result, err := tester.inv.Retry(test.funcToExecute)
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
			expectedOperation: string(model.OperationTypeCreated),
			expectedTxid:      "123456",
			msg: &kafka.Message{
				Key:   []byte(testMessageKey),
				Value: []byte(testCreateOrUpdateMessage),
			},
			relationsEnabled: true,
		},
		{
			name:              "Update Operation",
			expectedOperation: string(model.OperationTypeUpdated),
			expectedTxid:      "123456",
			msg: &kafka.Message{
				Key:   []byte(testMessageKey),
				Value: []byte(testCreateOrUpdateMessage),
			},
			relationsEnabled: true,
		},
		{
			name:              "Delete Operation",
			expectedOperation: string(model.OperationTypeDeleted),
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
			expectedOperation: string(model.OperationTypeCreated),
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

			if (test.expectedOperation == string(model.OperationTypeCreated) || test.expectedOperation == string(model.OperationTypeUpdated)) && test.relationsEnabled {
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
