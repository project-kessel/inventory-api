package resources

import (
	"testing"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/project-kessel/inventory-api/internal/biz"
	"github.com/project-kessel/inventory-api/internal/biz/model"
	"github.com/project-kessel/inventory-api/internal/data"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCalculateTuples(t *testing.T) {
	tests := []struct {
		name                   string
		version                uint
		currentWorkspaceID     string
		previousWorkspaceID    string
		expectTuplesToCreate   bool
		expectTuplesToDelete   bool
		expectedCreateResource string
		expectedDeleteResource string
		expectedCreateSubject  string
		expectedDeleteSubject  string
	}{
		{
			name:                   "version 0 creates initial tuple",
			version:                0,
			currentWorkspaceID:     "workspace-initial",
			previousWorkspaceID:    "",
			expectTuplesToCreate:   true,
			expectTuplesToDelete:   false,
			expectedCreateResource: "hbi:test-resource",
			expectedCreateSubject:  "workspace-initial",
		},
		{
			name:                   "workspace change creates and deletes tuples",
			version:                2,
			currentWorkspaceID:     "workspace-new",
			previousWorkspaceID:    "workspace-old",
			expectTuplesToCreate:   true,
			expectTuplesToDelete:   true,
			expectedCreateResource: "hbi:test-resource",
			expectedDeleteResource: "hbi:test-resource",
			expectedCreateSubject:  "workspace-new",
			expectedDeleteSubject:  "workspace-old",
		},
		{
			name:                   "workspace change creates and deletes tuples version 1",
			version:                1,
			currentWorkspaceID:     "workspace-new",
			previousWorkspaceID:    "workspace-old",
			expectTuplesToCreate:   true,
			expectTuplesToDelete:   true,
			expectedCreateResource: "hbi:test-resource",
			expectedDeleteResource: "hbi:test-resource",
			expectedCreateSubject:  "workspace-new",
			expectedDeleteSubject:  "workspace-old",
		},
		{
			name:                 "same workspace does not create or delete tuples",
			version:              2,
			currentWorkspaceID:   "workspace-same",
			previousWorkspaceID:  "workspace-same",
			expectTuplesToCreate: false,
			expectTuplesToDelete: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := data.NewFakeResourceRepositoryWithWorkspaceOverrides(tt.currentWorkspaceID, tt.previousWorkspaceID)
			sc := NewSchemaUsecase(repo, log.NewHelper(log.DefaultLogger))
			key, err := model.NewReporterResourceKey(
				model.LocalResourceId("test-resource"),
				model.ResourceType("host"),
				model.ReporterType("HBI"),
				model.ReporterInstanceId("test-instance"),
			)
			require.NoError(t, err)

			version := model.Version(tt.version)
			tupleEvent, err := model.NewTupleEvent(key, biz.OperationTypeCreated, &version, nil)
			require.NoError(t, err)

			result, err := sc.CalculateTuples(tupleEvent)
			require.NoError(t, err)
			assert.Equal(t, tt.expectTuplesToCreate, result.HasTuplesToCreate())
			assert.Equal(t, tt.expectTuplesToDelete, result.HasTuplesToDelete())

			if tt.expectTuplesToCreate {
				require.NotNil(t, result.TuplesToCreate())
				require.Len(t, *result.TuplesToCreate(), 1)
				createTuple := (*result.TuplesToCreate())[0]

				resource := createTuple.Resource()
				expectedResourceStr := tt.expectedCreateResource
				actualResourceStr := resource.Type().Namespace() + ":" + resource.Id().Serialize()
				assert.Equal(t, expectedResourceStr, actualResourceStr)

				subject := createTuple.Subject()
				expectedSubjectStr := tt.expectedCreateSubject
				actualSubjectStr := subject.Subject().Id().Serialize()
				assert.Equal(t, expectedSubjectStr, actualSubjectStr)
			}

			if tt.expectTuplesToDelete {
				require.NotNil(t, result.TuplesToDelete())
				require.Len(t, *result.TuplesToDelete(), 1)
				deleteTuple := (*result.TuplesToDelete())[0]

				resource := deleteTuple.Resource()
				expectedResourceStr := tt.expectedDeleteResource
				actualResourceStr := resource.Type().Namespace() + ":" + resource.Id().Serialize()
				assert.Equal(t, expectedResourceStr, actualResourceStr)

				subject := deleteTuple.Subject()
				expectedSubjectStr := tt.expectedDeleteSubject
				actualSubjectStr := subject.Subject().Id().Serialize()
				assert.Equal(t, expectedSubjectStr, actualSubjectStr)
			}
		})
	}
}

func TestGetWorkspaceVersions(t *testing.T) {
	repo := data.NewFakeResourceRepository()
	sc := NewSchemaUsecase(repo, log.NewHelper(log.DefaultLogger))

	key, err := model.NewReporterResourceKey(
		model.LocalResourceId("test-resource"),
		model.ResourceType("host"),
		model.ReporterType("HBI"),
		model.ReporterInstanceId("test-instance"),
	)
	require.NoError(t, err)

	result, err := sc.getWorkspaceVersions(key, 1)
	require.NoError(t, err)
	assert.NotEmpty(t, result)
}

func TestCreateWorkspaceTuple(t *testing.T) {
	key, err := model.NewReporterResourceKey(
		model.LocalResourceId("test-resource"),
		model.ResourceType("host"),
		model.ReporterType("HBI"),
		model.ReporterInstanceId("test-instance"),
	)
	require.NoError(t, err)

	tests := []struct {
		name        string
		workspaceID string
		validate    func(t *testing.T, tuple model.RelationsTuple)
	}{
		{
			name:        "normal workspace ID",
			workspaceID: "workspace-123",
			validate: func(t *testing.T, tuple model.RelationsTuple) {
				assert.IsType(t, model.RelationsTuple{}, tuple)

				resource := tuple.Resource()
				assert.Equal(t, "test-resource", resource.Id().String())
				assert.Equal(t, "host", resource.Type().Name())
				assert.Equal(t, "hbi", resource.Type().Namespace())

				assert.Equal(t, "workspace", tuple.Relation())

				subject := tuple.Subject()
				subjectResource := subject.Subject()
				assert.Equal(t, "workspace-123", subjectResource.Id().String())
				assert.Equal(t, "workspace", subjectResource.Type().Name())
				assert.Equal(t, "rbac", subjectResource.Type().Namespace())
			},
		},
		{
			name:        "workspace ID with special characters",
			workspaceID: "workspace-with-dashes_and_underscores",
			validate: func(t *testing.T, tuple model.RelationsTuple) {
				subject := tuple.Subject()
				subjectResource := subject.Subject()
				assert.Equal(t, "workspace-with-dashes_and_underscores", subjectResource.Id().String())
				assert.Equal(t, "workspace", subjectResource.Type().Name())
				assert.Equal(t, "rbac", subjectResource.Type().Namespace())
			},
		},
		{
			name:        "empty workspace ID",
			workspaceID: "",
			validate: func(t *testing.T, tuple model.RelationsTuple) {
				subject := tuple.Subject()
				subjectResource := subject.Subject()
				assert.Equal(t, "", subjectResource.Id().String())
				assert.Equal(t, "workspace", subjectResource.Type().Name())
				assert.Equal(t, "rbac", subjectResource.Type().Namespace())
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tuple := model.NewWorkspaceRelationsTuple(tt.workspaceID, key)
			tt.validate(t, tuple)
		})
	}
}

func TestDetermineTupleOperations(t *testing.T) {
	repo := data.NewFakeResourceRepository()
	sc := NewSchemaUsecase(repo, log.NewHelper(log.DefaultLogger))

	key, err := model.NewReporterResourceKey(
		model.LocalResourceId("test-resource"),
		model.ResourceType("host"),
		model.ReporterType("HBI"),
		model.ReporterInstanceId("test-instance"),
	)
	require.NoError(t, err)

	representationVersion := []data.RepresentationsByVersion{
		{
			Version: 2,
			Data: map[string]interface{}{
				"workspace_id": "workspace-new",
			},
		},
		{
			Version: 1,
			Data: map[string]interface{}{
				"workspace_id": "workspace-old",
			},
		},
	}

	result, err := sc.determineTupleOperations(representationVersion, 2, key)
	require.NoError(t, err)

	assert.True(t, result.HasTuplesToCreate() || result.HasTuplesToDelete())
}
