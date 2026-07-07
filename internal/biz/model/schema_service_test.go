package model_test

import (
	"context"
	"testing"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/project-kessel/inventory-api/internal/biz/model"
	"github.com/project-kessel/inventory-api/internal/data"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCalculateTuples(t *testing.T) {
	tests := []struct {
		name                   string
		version                model.Version
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
			version:                model.NewVersion(0),
			currentWorkspaceID:     "workspace-initial",
			previousWorkspaceID:    "",
			expectTuplesToCreate:   true,
			expectTuplesToDelete:   false,
			expectedCreateResource: "hbi:test-resource",
			expectedCreateSubject:  "workspace-initial",
		},
		{
			name:                   "workspace change creates and deletes tuples",
			version:                model.NewVersion(2),
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
			version:                model.NewVersion(1),
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
			version:              model.NewVersion(2),
			currentWorkspaceID:   "workspace-same",
			previousWorkspaceID:  "workspace-same",
			expectTuplesToCreate: false,
			expectTuplesToDelete: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sc := model.NewSchemaService(data.NewInMemorySchemaRepository(), log.NewHelper(log.DefaultLogger))
			resourceType, err := model.NewResourceType("host")
			require.NoError(t, err)
			reporterType, err := model.NewReporterType("HBI")
			require.NoError(t, err)
			reporterInstanceId, err := model.NewReporterInstanceId("test-instance")
			require.NoError(t, err)
			key, err := model.NewReporterResourceKey(
				model.LocalResourceId("test-resource"),
				resourceType, reporterType, reporterInstanceId,
			)
			require.NoError(t, err)

			// Build representations input
			var current, previous *model.Representations

			currentData := map[string]interface{}{}
			if tt.currentWorkspaceID != "" {
				currentData = map[string]interface{}{"workspace_id": tt.currentWorkspaceID}
			}
			ver := tt.version
			current, err = model.NewRepresentations(
				model.Representation(currentData),
				&ver,
				nil,
				nil,
			)
			require.NoError(t, err)

			if tt.previousWorkspaceID != "" {
				prevVer := tt.version.Decrement()
				previous, err = model.NewRepresentations(
					model.Representation(map[string]interface{}{"workspace_id": tt.previousWorkspaceID}),
					&prevVer,
					nil,
					nil,
				)
				require.NoError(t, err)
			}

			result, err := sc.CalculateTuplesForResource(context.Background(), current, previous, key)
			require.NoError(t, err)
			assert.Equal(t, tt.expectTuplesToCreate, result.HasTuplesToCreate())
			assert.Equal(t, tt.expectTuplesToDelete, result.HasTuplesToDelete())

			if tt.expectTuplesToCreate {
				require.NotNil(t, result.TuplesToCreate())
				require.Len(t, *result.TuplesToCreate(), 1)
				createTuple := (*result.TuplesToCreate())[0]

				resource := createTuple.Object()
				expectedResourceStr := tt.expectedCreateResource
				actualResourceStr := resource.Reporter().ReporterType().Serialize() + ":" + resource.ResourceId().Serialize()
				assert.Equal(t, expectedResourceStr, actualResourceStr)

				subject := createTuple.Subject().Resource()
				expectedSubjectStr := tt.expectedCreateSubject
				actualSubjectStr := subject.ResourceId().Serialize()
				assert.Equal(t, expectedSubjectStr, actualSubjectStr)
			}

			if tt.expectTuplesToDelete {
				require.NotNil(t, result.TuplesToDelete())
				require.Len(t, *result.TuplesToDelete(), 1)
				deleteTuple := (*result.TuplesToDelete())[0]

				resource := deleteTuple.Object()
				expectedResourceStr := tt.expectedDeleteResource
				actualResourceStr := resource.Reporter().ReporterType().Serialize() + ":" + resource.ResourceId().Serialize()
				assert.Equal(t, expectedResourceStr, actualResourceStr)

				subject := deleteTuple.Subject().Resource()
				expectedSubjectStr := tt.expectedDeleteSubject
				actualSubjectStr := subject.ResourceId().Serialize()
				assert.Equal(t, expectedSubjectStr, actualSubjectStr)
			}
		})
	}
}

func TestGetWorkspaceVersions(t *testing.T) {
	sc := model.NewSchemaService(data.NewInMemorySchemaRepository(), log.NewHelper(log.DefaultLogger))

	resourceType, err := model.NewResourceType("host")
	require.NoError(t, err)
	reporterType, err := model.NewReporterType("HBI")
	require.NoError(t, err)
	reporterInstanceId, err := model.NewReporterInstanceId("test-instance")
	require.NoError(t, err)
	key, err := model.NewReporterResourceKey(
		model.LocalResourceId("test-resource"),
		resourceType, reporterType, reporterInstanceId,
	)
	require.NoError(t, err)

	ver := model.NewVersion(1)
	current, err := model.NewRepresentations(
		model.Representation(map[string]interface{}{"workspace_id": "ws-current"}),
		&ver,
		nil,
		nil,
	)
	require.NoError(t, err)
	prevVer := ver.Decrement()
	previous, err := model.NewRepresentations(
		model.Representation(map[string]interface{}{"workspace_id": "ws-prev"}),
		&prevVer,
		nil,
		nil,
	)
	require.NoError(t, err)
	result, err := sc.CalculateTuplesForResource(context.Background(), current, previous, key)
	require.NoError(t, err)
	assert.True(t, result.HasTuplesToCreate() || result.HasTuplesToDelete())
}

func TestCreateWorkspaceTuple(t *testing.T) {
	resourceType, err := model.NewResourceType("host")
	require.NoError(t, err)
	reporterType, err := model.NewReporterType("HBI")
	require.NoError(t, err)
	reporterInstanceId, err := model.NewReporterInstanceId("test-instance")
	require.NoError(t, err)
	key, err := model.NewReporterResourceKey(
		model.LocalResourceId("test-resource"),
		resourceType, reporterType, reporterInstanceId,
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

				resource := tuple.Object()
				assert.Equal(t, "test-resource", resource.ResourceId().String())
				assert.Equal(t, "host", resource.ResourceType().Serialize())
				assert.Equal(t, "hbi", resource.Reporter().ReporterType().Serialize())

				assert.Equal(t, model.DeserializeRelation("workspace"), tuple.Relation())

				subjectResource := tuple.Subject().Resource()
				assert.Equal(t, "workspace-123", subjectResource.ResourceId().String())
				assert.Equal(t, "workspace", subjectResource.ResourceType().Serialize())
				assert.Equal(t, "rbac", subjectResource.Reporter().ReporterType().Serialize())
			},
		},
		{
			name:        "workspace ID with special characters",
			workspaceID: "workspace-with-dashes_and_underscores",
			validate: func(t *testing.T, tuple model.RelationsTuple) {
				subjectResource := tuple.Subject().Resource()
				assert.Equal(t, "workspace-with-dashes_and_underscores", subjectResource.ResourceId().String())
				assert.Equal(t, "workspace", subjectResource.ResourceType().Serialize())
				assert.Equal(t, "rbac", subjectResource.Reporter().ReporterType().Serialize())
			},
		},
		{
			name:        "empty workspace ID",
			workspaceID: "",
			validate: func(t *testing.T, tuple model.RelationsTuple) {
				subjectResource := tuple.Subject().Resource()
				assert.Equal(t, "", subjectResource.ResourceId().String())
				assert.Equal(t, "workspace", subjectResource.ResourceType().Serialize())
				assert.Equal(t, "rbac", subjectResource.Reporter().ReporterType().Serialize())
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
	sc := model.NewSchemaService(data.NewInMemorySchemaRepository(), log.NewHelper(log.DefaultLogger))

	resourceType, err := model.NewResourceType("host")
	require.NoError(t, err)
	reporterType, err := model.NewReporterType("HBI")
	require.NoError(t, err)
	reporterInstanceId, err := model.NewReporterInstanceId("test-instance")
	require.NoError(t, err)
	key, err := model.NewReporterResourceKey(
		model.LocalResourceId("test-resource"),
		resourceType, reporterType, reporterInstanceId,
	)
	require.NoError(t, err)

	ver2 := model.NewVersion(2)
	current, err := model.NewRepresentations(
		model.Representation(map[string]interface{}{"workspace_id": "workspace-new"}),
		&ver2,
		nil,
		nil,
	)
	require.NoError(t, err)

	ver1 := model.NewVersion(1)
	previous, err := model.NewRepresentations(
		model.Representation(map[string]interface{}{"workspace_id": "workspace-old"}),
		&ver1,
		nil,
		nil,
	)
	require.NoError(t, err)

	result, err := sc.CalculateTuplesForResource(context.Background(), current, previous, key)
	require.NoError(t, err)

	assert.True(t, result.HasTuplesToCreate() || result.HasTuplesToDelete())
}

func TestCalculateTuples_OperationTypeScenarios(t *testing.T) {
	testCases := []struct {
		name                 string
		operationType        model.EventOperationType // kept for scenario naming; not used by CalculateTuples
		version              model.Version
		currentWorkspaceID   string
		previousWorkspaceID  string
		expectTuplesToCreate bool
		expectTuplesToDelete bool
	}{
		{
			name:                 "CREATE operation should only create tuples",
			operationType:        model.OperationTypeCreated,
			version:              model.NewVersion(0),
			currentWorkspaceID:   "workspace-new",
			previousWorkspaceID:  "",
			expectTuplesToCreate: true,
			expectTuplesToDelete: false,
		},
		{
			name:                 "UPDATE operation with workspace change should create and delete tuples",
			operationType:        model.OperationTypeUpdated,
			version:              model.NewVersion(1),
			currentWorkspaceID:   "workspace-new",
			previousWorkspaceID:  "workspace-old",
			expectTuplesToCreate: true,
			expectTuplesToDelete: true,
		},
		{
			name:                 "UPDATE operation with same workspace should not create or delete tuples",
			operationType:        model.OperationTypeUpdated,
			version:              model.NewVersion(1),
			currentWorkspaceID:   "workspace-same",
			previousWorkspaceID:  "workspace-same",
			expectTuplesToCreate: false,
			expectTuplesToDelete: false,
		},
		{
			name:                 "DELETE operation should only delete tuples",
			operationType:        model.OperationTypeDeleted,
			version:              model.NewVersion(1),
			currentWorkspaceID:   "",                  // synthetic empty current
			previousWorkspaceID:  "workspace-current", // previous holds latest
			expectTuplesToCreate: false,
			expectTuplesToDelete: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			sc := model.NewSchemaService(data.NewInMemorySchemaRepository(), log.NewHelper(log.DefaultLogger))

			resourceType, err := model.NewResourceType("host")
			require.NoError(t, err)
			reporterType, err := model.NewReporterType("HBI")
			require.NoError(t, err)
			reporterInstanceId, err := model.NewReporterInstanceId("test-instance")
			require.NoError(t, err)
			key, err := model.NewReporterResourceKey(
				model.LocalResourceId("test-resource"),
				resourceType, reporterType, reporterInstanceId,
			)
			require.NoError(t, err)

			// Build representations to reflect the scenario
			var current, previous *model.Representations

			// Build current representation
			if tc.currentWorkspaceID != "" {
				currentData := map[string]interface{}{"workspace_id": tc.currentWorkspaceID}
				curVer := tc.version
				currentRep, err := model.NewRepresentations(
					model.Representation(currentData),
					&curVer,
					nil,
					nil,
				)
				require.NoError(t, err)
				current = currentRep
			} else {
				current = nil
			}

			if tc.previousWorkspaceID != "" {
				prevVer := tc.version.Decrement()
				previous, err = model.NewRepresentations(
					model.Representation(map[string]interface{}{"workspace_id": tc.previousWorkspaceID}),
					&prevVer,
					nil,
					nil,
				)
				require.NoError(t, err)
			}

			result, err := sc.CalculateTuplesForResource(context.Background(), current, previous, key)
			require.NoError(t, err)

			// Verify tuple creation expectations
			assert.Equal(t, tc.expectTuplesToCreate, result.HasTuplesToCreate(),
				"Operation %s should have expectTuplesToCreate=%v", tc.operationType.OperationType(), tc.expectTuplesToCreate)

			// Verify tuple deletion expectations
			assert.Equal(t, tc.expectTuplesToDelete, result.HasTuplesToDelete(),
				"Operation %s should have expectTuplesToDelete=%v", tc.operationType.OperationType(), tc.expectTuplesToDelete)

			// Additional validations based on operation type
			switch tc.operationType.OperationType() {
			case model.OperationTypeCreated:
				// For CREATE operations, check if delete tuples are actually empty
				if result.HasTuplesToDelete() {
					deleteTuples := result.TuplesToDelete()
					if deleteTuples != nil && len(*deleteTuples) > 0 {
						t.Logf("CREATE operation has %d delete tuples: %+v", len(*deleteTuples), *deleteTuples)
						// Log the workspace IDs to understand what's being deleted
						for i, tuple := range *deleteTuples {
							t.Logf("Delete tuple %d: resource=%s, subject=%s", i,
								tuple.Object().ResourceId().Serialize(),
								tuple.Subject().Resource().ResourceId().Serialize())
						}
					} else {
						t.Logf("CREATE operation has TuplesToDelete=true but slice is empty or nil")
					}
				} else {
					t.Logf("CREATE operation has no delete tuples (as expected)")
				}

				if tc.currentWorkspaceID != "" {
					assert.True(t, result.HasTuplesToCreate(), "CREATE operations should create tuples when workspace exists")
				}

			case model.OperationTypeUpdated:
				// UPDATE behavior depends on workspace changes
				if tc.currentWorkspaceID != tc.previousWorkspaceID && tc.currentWorkspaceID != "" && tc.previousWorkspaceID != "" {
					assert.True(t, result.HasTuplesToCreate(), "UPDATE with workspace change should create new tuple")
					assert.True(t, result.HasTuplesToDelete(), "UPDATE with workspace change should delete old tuple")
				} else if tc.currentWorkspaceID == tc.previousWorkspaceID {
					assert.False(t, result.HasTuplesToCreate(), "UPDATE with same workspace should not create tuples")
					assert.False(t, result.HasTuplesToDelete(), "UPDATE with same workspace should not delete tuples")
				}

			case model.OperationTypeDeleted:
				// DELETE should never create tuples
				assert.False(t, result.HasTuplesToCreate(), "DELETE operations should never create tuples")
				if tc.currentWorkspaceID != "" {
					assert.True(t, result.HasTuplesToDelete(), "DELETE operations should delete tuples when workspace exists")
				}
			}
		})
	}
}

func newRelationDef(t *testing.T, fieldName, relationName, subjectNamespace, subjectResourceType string, multiValued bool) model.RelationDef {
	t.Helper()
	rd, err := model.NewRelationDef(fieldName, relationName, subjectNamespace, subjectResourceType, multiValued)
	require.NoError(t, err)
	return rd
}

func TestCalculateTuplesFromRelationDefs(t *testing.T) {
	type expectedTuple struct {
		relation    string
		subjectId   string
		subjectType string
		namespace   string
	}

	allRelations := []model.RelationDef{
		newRelationDef(t, "allowed_workspaces", "allowed_workspaces", "rbac", "workspace", true),
		newRelationDef(t, "billing_account", "billing_account", "features", "billing_account", true),
		newRelationDef(t, "parent", "parent", "features", "service", false),
	}

	tests := []struct {
		name                 string
		relations            []model.RelationDef
		currentData          map[string]interface{}
		previousData         map[string]interface{}
		expectTuplesToCreate bool
		expectTuplesToDelete bool
		expectedCreates      []expectedTuple
		expectedDeletes      []expectedTuple
	}{
		{
			name:      "initial creation produces creates for all relations",
			relations: allRelations,
			currentData: map[string]interface{}{
				"allowed_workspaces": []interface{}{"ws-1", "ws-2"},
				"billing_account":    []interface{}{"ba-1"},
				"parent":             "parent-svc",
			},
			previousData:         nil,
			expectTuplesToCreate: true,
			expectTuplesToDelete: false,
			expectedCreates: []expectedTuple{
				{"allowed_workspaces", "ws-1", "workspace", "rbac"},
				{"allowed_workspaces", "ws-2", "workspace", "rbac"},
				{"billing_account", "ba-1", "billing_account", "features"},
				{"parent", "parent-svc", "service", "features"},
			},
		},
		{
			name:      "multi-valued update creates and deletes changed values",
			relations: allRelations[:2],
			currentData: map[string]interface{}{
				"allowed_workspaces": []interface{}{"ws-2", "ws-3"},
				"billing_account":    []interface{}{"ba-1"},
			},
			previousData: map[string]interface{}{
				"allowed_workspaces": []interface{}{"ws-1", "ws-2"},
				"billing_account":    []interface{}{"ba-1"},
			},
			expectTuplesToCreate: true,
			expectTuplesToDelete: true,
			expectedCreates: []expectedTuple{
				{"allowed_workspaces", "ws-3", "workspace", "rbac"},
			},
			expectedDeletes: []expectedTuple{
				{"allowed_workspaces", "ws-1", "workspace", "rbac"},
			},
		},
		{
			name: "scalar field change creates new and deletes old",
			relations: []model.RelationDef{
				newRelationDef(t, "parent", "parent", "features", "service", false),
			},
			currentData:          map[string]interface{}{"parent": "new-parent"},
			previousData:         map[string]interface{}{"parent": "old-parent"},
			expectTuplesToCreate: true,
			expectTuplesToDelete: true,
			expectedCreates: []expectedTuple{
				{"parent", "new-parent", "service", "features"},
			},
			expectedDeletes: []expectedTuple{
				{"parent", "old-parent", "service", "features"},
			},
		},
		{
			name:      "same data produces no tuples",
			relations: allRelations,
			currentData: map[string]interface{}{
				"allowed_workspaces": []interface{}{"ws-1"},
				"parent":             "parent-svc",
			},
			previousData: map[string]interface{}{
				"allowed_workspaces": []interface{}{"ws-1"},
				"parent":             "parent-svc",
			},
			expectTuplesToCreate: false,
			expectTuplesToDelete: false,
		},
		{
			name:        "nil current produces only deletes",
			relations:   allRelations,
			currentData: nil,
			previousData: map[string]interface{}{
				"allowed_workspaces": []interface{}{"ws-1"},
				"parent":             "parent-svc",
			},
			expectTuplesToCreate: false,
			expectTuplesToDelete: true,
			expectedDeletes: []expectedTuple{
				{"allowed_workspaces", "ws-1", "workspace", "rbac"},
				{"parent", "parent-svc", "service", "features"},
			},
		},
		{
			name:                 "empty relations produces no tuples",
			relations:            nil,
			currentData:          map[string]interface{}{"some_field": "value"},
			previousData:         nil,
			expectTuplesToCreate: false,
			expectTuplesToDelete: false,
		},
		{
			name: "missing field in representation produces no tuples",
			relations: []model.RelationDef{
				newRelationDef(t, "parent", "parent", "features", "service", false),
			},
			currentData:          map[string]interface{}{"unrelated_field": "value"},
			previousData:         nil,
			expectTuplesToCreate: false,
			expectTuplesToDelete: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key := newTestReporterResourceKey(t)

			var current, previous *model.Representations

			if tt.currentData != nil {
				ver := model.NewVersion(1)
				c, err := model.NewRepresentations(model.Representation(tt.currentData), &ver, nil, nil)
				require.NoError(t, err)
				current = c
			}

			if tt.previousData != nil {
				prevVer := model.NewVersion(0)
				p, err := model.NewRepresentations(model.Representation(tt.previousData), &prevVer, nil, nil)
				require.NoError(t, err)
				previous = p
			}

			result, err := model.CalculateTuplesFromRelationDefs(tt.relations, current, previous, key)
			require.NoError(t, err)

			assert.Equal(t, tt.expectTuplesToCreate, result.HasTuplesToCreate())
			assert.Equal(t, tt.expectTuplesToDelete, result.HasTuplesToDelete())

			if tt.expectTuplesToCreate {
				creates := *result.TuplesToCreate()
				assert.Len(t, creates, len(tt.expectedCreates))
				for _, exp := range tt.expectedCreates {
					found := false
					for _, tuple := range creates {
						if tuple.Relation().Serialize() == exp.relation &&
							tuple.Subject().Resource().ResourceId().Serialize() == exp.subjectId &&
							tuple.Subject().Resource().ResourceType().Serialize() == exp.subjectType &&
							tuple.Subject().Resource().Reporter().ReporterType().Serialize() == exp.namespace {
							found = true
							break
						}
					}
					assert.True(t, found, "expected create tuple %+v not found", exp)
				}
			}

			if tt.expectTuplesToDelete {
				deletes := *result.TuplesToDelete()
				assert.Len(t, deletes, len(tt.expectedDeletes))
				for _, exp := range tt.expectedDeletes {
					found := false
					for _, tuple := range deletes {
						if tuple.Relation().Serialize() == exp.relation &&
							tuple.Subject().Resource().ResourceId().Serialize() == exp.subjectId &&
							tuple.Subject().Resource().ResourceType().Serialize() == exp.subjectType &&
							tuple.Subject().Resource().Reporter().ReporterType().Serialize() == exp.namespace {
							found = true
							break
						}
					}
					assert.True(t, found, "expected delete tuple %+v not found", exp)
				}
			}
		})
	}
}

func TestValidateReportAgainstSchema(t *testing.T) {
	ctx := context.Background()

	resourceType, err := model.NewResourceType("host")
	require.NoError(t, err)
	reporterType, err := model.NewReporterType("HBI")
	require.NoError(t, err)
	unknownReporterType, err := model.NewReporterType("UNKNOWN")
	require.NoError(t, err)

	commonSchema := data.NewJsonSchemaWithWorkspacesFromString(`{
		"type": "object",
		"properties": { "workspace_id": { "type": "string" } },
		"required": ["workspace_id"]
	}`)
	reporterSchema := data.NewJsonSchemaWithWorkspacesFromString(`{
		"type": "object",
		"properties": { "satellite_id": { "type": "string" } },
		"required": ["satellite_id"]
	}`)

	setupRepo := func() *data.InMemorySchemaRepository {
		repo := data.NewInMemorySchemaRepository()
		resourceRep, err := model.NewResourceSchemaRepresentation(resourceType, commonSchema)
		require.NoError(t, err)
		require.NoError(t, repo.CreateResourceSchema(ctx, resourceRep))
		reporterRep, err := model.NewReporterSchemaRepresentation(resourceType, reporterType, reporterSchema)
		require.NoError(t, err)
		require.NoError(t, repo.CreateReporterSchema(ctx, reporterRep))
		return repo
	}

	t.Run("reporter not allowed for resource type", func(t *testing.T) {
		sc := model.NewSchemaService(setupRepo(), log.NewHelper(log.DefaultLogger))
		err := sc.ValidateReportAgainstSchema(ctx, resourceType, unknownReporterType, nil, nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "does not report resource types")
	})

	t.Run("invalid reporter representation fails validation", func(t *testing.T) {
		sc := model.NewSchemaService(setupRepo(), log.NewHelper(log.DefaultLogger))
		invalidReporter := model.Representation(map[string]interface{}{"wrong_field": "value"})
		validCommon := model.Representation(map[string]interface{}{"workspace_id": "ws-1"})
		err := sc.ValidateReportAgainstSchema(ctx, resourceType, reporterType, &validCommon, &invalidReporter)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "satellite_id is required")
	})

	t.Run("invalid common representation fails validation", func(t *testing.T) {
		sc := model.NewSchemaService(setupRepo(), log.NewHelper(log.DefaultLogger))
		validReporter := model.Representation(map[string]interface{}{"satellite_id": "sat-1"})
		invalidCommon := model.Representation(map[string]interface{}{"wrong_field": "value"})
		err := sc.ValidateReportAgainstSchema(ctx, resourceType, reporterType, &invalidCommon, &validReporter)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "workspace_id is required")
	})

	t.Run("both representations nil passes", func(t *testing.T) {
		sc := model.NewSchemaService(setupRepo(), log.NewHelper(log.DefaultLogger))
		err := sc.ValidateReportAgainstSchema(ctx, resourceType, reporterType, nil, nil)
		assert.NoError(t, err)
	})

	t.Run("both representations valid passes", func(t *testing.T) {
		sc := model.NewSchemaService(setupRepo(), log.NewHelper(log.DefaultLogger))
		validCommon := model.Representation(map[string]interface{}{"workspace_id": "ws-1"})
		validReporter := model.Representation(map[string]interface{}{"satellite_id": "sat-1"})
		err := sc.ValidateReportAgainstSchema(ctx, resourceType, reporterType, &validCommon, &validReporter)
		assert.NoError(t, err)
	})
}
