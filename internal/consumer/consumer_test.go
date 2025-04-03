package consumer

import (
	"testing"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/project-kessel/inventory-api/cmd/common"
	"github.com/project-kessel/inventory-api/internal/authz"
	"github.com/project-kessel/inventory-api/internal/pubsub"
	"github.com/project-kessel/relations-api/api/kessel/relations/v1beta1"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/otel"
	"gorm.io/gorm"
)

type TestCase struct {
	name            string
	description     string
	options         *Options
	config          *Config
	completedConfig CompletedConfig
	inv             InventoryConsumer
	metrics         MetricsCollector
	logger          *log.Helper
}

// TestSetup creates a test struct that calls most of the initial constructor methods we intend to test in unit tests.
func (t *TestCase) TestSetup() []error {
	t.options = NewOptions()
	t.options.BootstrapServers = []string{"localhost:9092"}
	t.config = NewConfig(t.options)

	_, logger := common.InitLogger("info", common.LoggerOptions{})
	t.logger = log.NewHelper(log.With(logger, "subsystem", "inventoryConsumer"))

	var errs []error
	var err error

	errs = t.options.Complete()
	errs = t.options.Validate()
	t.completedConfig, errs = NewConfig(t.options).Complete()

	notifier := &pubsub.NotifierMock{}

	t.inv, err = New(t.completedConfig, &gorm.DB{}, authz.CompletedConfig{}, nil, notifier, t.logger)
	if err != nil {
		errs = append(errs, err)
	}
	err = t.metrics.New(otel.Meter("github.com/project-kessel/inventory-api/blob/main/internal/server/otel"))
	if err != nil {
		errs = append(errs, err)
	}
	return errs
}

func TestNewConsumerSetup(t *testing.T) {
	test := TestCase{
		name:        "TestNewConsumerSetup",
		description: "ensures setting up a new consumer, including options and configs functions",
	}
	var errs []error
	errs = test.TestSetup()
	assert.Nil(t, errs)
}

func TestParseCreateOrUpdateMessage(t *testing.T) {
	testMsg := `{"schema":{"type":"string","optional":false,"name":"io.debezium.data.Json","version":1},"payload":"{\"subject\":{\"subject\":{\"id\":\"1234\", \"type\":{\"name\":\"workspace\",\"namespace\":\"rbac\"}}},\"relation\":\"t_workspace\",\"resource\":{\"id\":\"4321\",\"type\":{\"name\":\"integration\",\"namespace\":\"notifications\"}}}"}`
	expected := &v1beta1.Relationship{
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
	tuple, err := ParseCreateOrUpdateMessage([]byte(testMsg))
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
	testMsg := `{"schema":{"type":"string","optional":false,"name":"io.debezium.data.Json","version":1},"payload":"{\"resource_id\":\"4321\",\"resource_type\":\"integration\",\"resource_namespace\":\"notifications\",\"relation\":\"t_workspace\",\"subject_filter\":{\"subject_type\":\"workspace\",\"subject_namespace\":\"rbac\",\"subject_id\":\"1234\"}}"}`
	resourceNamespace := "notifications"
	resourceType := "integration"
	resourceId := "4321"
	relation := "t_workspace"
	subjectType := "workspace"
	subjectNamespace := "rbac"
	subjectId := "1234"

	expected := &v1beta1.RelationTupleFilter{
		ResourceNamespace: &resourceNamespace,
		ResourceType:      &resourceType,
		ResourceId:        &resourceId,
		Relation:          &relation,
		SubjectFilter: &v1beta1.SubjectFilter{
			SubjectNamespace: &subjectNamespace,
			SubjectType:      &subjectType,
			SubjectId:        &subjectId,
		},
	}
	filter, err := ParseDeleteMessage([]byte(testMsg))
	assert.Nil(t, err)
	assert.Equal(t, filter.ResourceId, expected.ResourceId)
	assert.Equal(t, filter.ResourceType, expected.ResourceType)
	assert.Equal(t, filter.ResourceNamespace, expected.ResourceNamespace)
	assert.Equal(t, filter.Relation, expected.Relation)
	assert.Equal(t, filter.SubjectFilter.SubjectId, expected.SubjectFilter.SubjectId)
	assert.Equal(t, filter.SubjectFilter.SubjectType, expected.SubjectFilter.SubjectType)
	assert.Equal(t, filter.SubjectFilter.SubjectNamespace, expected.SubjectFilter.SubjectNamespace)
}

func TestParseMessageKey(t *testing.T) {
	testMsg := `{"schema":{"type":"string","optional":false},"payload":"00000000-0000-0000-0000-000000000000"}`
	expected := "00000000-0000-0000-0000-000000000000"
	key, err := ParseMessageKey([]byte(testMsg))
	assert.Nil(t, err)
	assert.Equal(t, key, expected)
}
