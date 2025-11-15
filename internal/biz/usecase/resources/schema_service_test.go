package resources

import (
	"context"
	"testing"

	"github.com/project-kessel/inventory-api/internal/biz/schema"
	"github.com/project-kessel/inventory-api/internal/biz/schema/validation"

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
			sc := NewSchemaUsecase(data.NewInMemorySchemaRepository(), log.NewHelper(log.DefaultLogger))
			key, err := model.NewReporterResourceKey(
				model.LocalResourceId("test-resource"),
				model.ResourceType("host"),
				model.ReporterType("HBI"),
				model.ReporterInstanceId("test-instance"),
			)
			require.NoError(t, err)

			// Build representations input
			var reps []data.RepresentationsByVersion
			if tt.currentWorkspaceID != "" {
				reps = append(reps, data.RepresentationsByVersion{
					Version: tt.version,
					Data: map[string]interface{}{
						"workspace_id": tt.currentWorkspaceID,
					},
				})
			} else {
				// Synthetic empty current (for delete-like or no-op scenarios)
				reps = append(reps, data.RepresentationsByVersion{
					Version: tt.version,
					Data:    map[string]interface{}{},
				})
			}
			if tt.previousWorkspaceID != "" {
				prevVer := uint(0)
				if tt.version > 0 {
					prevVer = tt.version - 1
				}
				reps = append(reps, data.RepresentationsByVersion{
					Version: prevVer,
					Data: map[string]interface{}{
						"workspace_id": tt.previousWorkspaceID,
					},
				})
			}

			result, err := sc.CalculateTuples(reps, key)
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
	sc := NewSchemaUsecase(data.NewInMemorySchemaRepository(), log.NewHelper(log.DefaultLogger))

	key, err := model.NewReporterResourceKey(
		model.LocalResourceId("test-resource"),
		model.ResourceType("host"),
		model.ReporterType("HBI"),
		model.ReporterInstanceId("test-instance"),
	)
	require.NoError(t, err)

	version := uint(1)
	reps := []data.RepresentationsByVersion{
		{Version: version, Data: map[string]interface{}{"workspace_id": "ws-current"}},
		{Version: version - 1, Data: map[string]interface{}{"workspace_id": "ws-prev"}},
	}
	result, err := sc.CalculateTuples(reps, key)
	require.NoError(t, err)
	assert.True(t, result.HasTuplesToCreate() || result.HasTuplesToDelete())
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
	sc := NewSchemaUsecase(data.NewInMemorySchemaRepository(), log.NewHelper(log.DefaultLogger))

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

	currentWorkspaceID, previousWorkspaceID := data.GetCurrentAndPreviousWorkspaceID(representationVersion, 2)
	result, err := sc.BuildTuplesToReplicate(currentWorkspaceID, previousWorkspaceID, key)
	require.NoError(t, err)

	assert.True(t, result.HasTuplesToCreate() || result.HasTuplesToDelete())
}

func TestCalculateTuples_OperationTypeScenarios(t *testing.T) {
	testCases := []struct {
		name                 string
		operationType        biz.EventOperationType // kept for scenario naming; not used by CalculateTuples
		version              uint
		currentWorkspaceID   string
		previousWorkspaceID  string
		expectTuplesToCreate bool
		expectTuplesToDelete bool
	}{
		{
			name:                 "CREATE operation should only create tuples",
			operationType:        biz.OperationTypeCreated,
			version:              0,
			currentWorkspaceID:   "workspace-new",
			previousWorkspaceID:  "",
			expectTuplesToCreate: true,
			expectTuplesToDelete: false,
		},
		{
			name:                 "UPDATE operation with workspace change should create and delete tuples",
			operationType:        biz.OperationTypeUpdated,
			version:              1,
			currentWorkspaceID:   "workspace-new",
			previousWorkspaceID:  "workspace-old",
			expectTuplesToCreate: true,
			expectTuplesToDelete: true,
		},
		{
			name:                 "UPDATE operation with same workspace should not create or delete tuples",
			operationType:        biz.OperationTypeUpdated,
			version:              1,
			currentWorkspaceID:   "workspace-same",
			previousWorkspaceID:  "workspace-same",
			expectTuplesToCreate: false,
			expectTuplesToDelete: false,
		},
		{
			name:                 "DELETE operation should only delete tuples",
			operationType:        biz.OperationTypeDeleted,
			version:              1,
			currentWorkspaceID:   "",                  // synthetic empty current
			previousWorkspaceID:  "workspace-current", // previous holds latest
			expectTuplesToCreate: false,
			expectTuplesToDelete: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			sc := NewSchemaUsecase(data.NewInMemorySchemaRepository(), log.NewHelper(log.DefaultLogger))

			key, err := model.NewReporterResourceKey(
				model.LocalResourceId("test-resource"),
				model.ResourceType("host"),
				model.ReporterType("HBI"),
				model.ReporterInstanceId("test-instance"),
			)
			require.NoError(t, err)

			// Build representations to reflect the scenario
			var reps []data.RepresentationsByVersion
			if tc.currentWorkspaceID != "" {
				reps = append(reps, data.RepresentationsByVersion{
					Version: tc.version,
					Data: map[string]interface{}{
						"workspace_id": tc.currentWorkspaceID,
					},
				})
			} else {
				// synthetic empty current (used to emulate delete)
				reps = append(reps, data.RepresentationsByVersion{
					Version: tc.version + 1, // ensure it's considered current
					Data:    map[string]interface{}{},
				})
			}
			if tc.previousWorkspaceID != "" {
				prevVer := uint(0)
				if tc.version > 0 {
					prevVer = tc.version - 1
				}
				reps = append(reps, data.RepresentationsByVersion{
					Version: prevVer,
					Data: map[string]interface{}{
						"workspace_id": tc.previousWorkspaceID,
					},
				})
			}

			result, err := sc.CalculateTuples(reps, key)
			require.NoError(t, err)

			// Verify tuple creation expectations
			assert.Equal(t, tc.expectTuplesToCreate, result.HasTuplesToCreate(),
				"Operation %s should have expectTuplesToCreate=%v", tc.operationType.OperationType(), tc.expectTuplesToCreate)

			// Verify tuple deletion expectations
			assert.Equal(t, tc.expectTuplesToDelete, result.HasTuplesToDelete(),
				"Operation %s should have expectTuplesToDelete=%v", tc.operationType.OperationType(), tc.expectTuplesToDelete)

			// Additional validations based on operation type
			switch tc.operationType.OperationType() {
			case biz.OperationTypeCreated:
				// For CREATE operations, check if delete tuples are actually empty
				if result.HasTuplesToDelete() {
					deleteTuples := result.TuplesToDelete()
					if deleteTuples != nil && len(*deleteTuples) > 0 {
						t.Logf("CREATE operation has %d delete tuples: %+v", len(*deleteTuples), *deleteTuples)
						// Log the workspace IDs to understand what's being deleted
						for i, tuple := range *deleteTuples {
							t.Logf("Delete tuple %d: resource=%s, subject=%s", i,
								tuple.Resource().Id().Serialize(),
								tuple.Subject().Subject().Id().Serialize())
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

			case biz.OperationTypeUpdated:
				// UPDATE behavior depends on workspace changes
				if tc.currentWorkspaceID != tc.previousWorkspaceID && tc.currentWorkspaceID != "" && tc.previousWorkspaceID != "" {
					assert.True(t, result.HasTuplesToCreate(), "UPDATE with workspace change should create new tuple")
					assert.True(t, result.HasTuplesToDelete(), "UPDATE with workspace change should delete old tuple")
				} else if tc.currentWorkspaceID == tc.previousWorkspaceID {
					assert.False(t, result.HasTuplesToCreate(), "UPDATE with same workspace should not create tuples")
					assert.False(t, result.HasTuplesToDelete(), "UPDATE with same workspace should not delete tuples")
				}

			case biz.OperationTypeDeleted:
				// DELETE should never create tuples
				assert.False(t, result.HasTuplesToCreate(), "DELETE operations should never create tuples")
				if tc.currentWorkspaceID != "" {
					assert.True(t, result.HasTuplesToDelete(), "DELETE operations should delete tuples when workspace exists")
				}
			}
		})
	}
}

func TestSchemaServiceImpl_ValidateReporterForResource(t *testing.T) {
	tests := []struct {
		name            string
		resourceType    string
		reporterType    string
		setupRepository func(repository schema.Repository)
		isReporter      bool
		expectErr       bool
		expectedError   string
	}{
		{
			name:         "Valid resource and reporter combination",
			resourceType: "host",
			reporterType: "hbi",
			setupRepository: func(repository schema.Repository) {
				err := repository.CreateResourceSchema(context.Background(), schema.ResourceRepresentation{
					ResourceType:     "host",
					ValidationSchema: validation.NewJsonSchemaValidatorFromString(`{"type": "object"}`),
				})

				assert.NoError(t, err)

				err = repository.CreateReporterSchema(context.Background(), schema.ReporterRepresentation{
					ResourceType:     "host",
					ReporterType:     "hbi",
					ValidationSchema: validation.NewJsonSchemaValidatorFromString(`{"type": "object"}`),
				})
				assert.NoError(t, err)
			},
			isReporter: true,
			expectErr:  false,
		},
		{
			name:         "Invalid resource and reporter combination",
			resourceType: "host",
			reporterType: "invalid_reporter",
			setupRepository: func(repository schema.Repository) {
				err := repository.CreateResourceSchema(context.Background(), schema.ResourceRepresentation{
					ResourceType:     "host",
					ValidationSchema: validation.NewJsonSchemaValidatorFromString(`{"type": "object"}`),
				})
				assert.NoError(t, err)
			},
			isReporter:    false,
			expectErr:     false,
			expectedError: "invalid reporter_type: invalid_reporter for resource_type: host",
		},
		{
			name:         "Resource type does not exist",
			resourceType: "invalid_resource",
			reporterType: "hbi",
			setupRepository: func(repository schema.Repository) {
				// nothing here
			},
			isReporter:    false,
			expectErr:     true,
			expectedError: "resource type invalid_resource does not exist",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeRepo := data.NewInMemorySchemaRepository()
			tt.setupRepository(fakeRepo)
			sc := NewSchemaUsecase(data.NewInMemorySchemaRepository(), log.NewHelper(log.DefaultLogger))
			ctx := context.Background()

			isReporter, err := sc.IsReporterForResource(ctx, tt.resourceType, tt.reporterType)

			assert.Equal(t, tt.isReporter, isReporter)

			if tt.expectErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestSchemaServiceImpl_CommonShallowValidate(t *testing.T) {
	validCommonSchema := `{
		"$schema": "http://json-schema.org/draft-07/schema#",
		"type": "object",
		"properties": {
			"workspace_id": { "type": "string" }
		},
		"required": ["workspace_id"]
	}`

	tests := []struct {
		name                 string
		resourceType         string
		commonRepresentation map[string]interface{}
		setupRepository      func(repository schema.Repository)
		expectErr            bool
		expectedError        string
	}{
		{
			name:         "Valid common representation",
			resourceType: "host",
			commonRepresentation: map[string]interface{}{
				"workspace_id": "ws-123",
			},
			setupRepository: func(repository schema.Repository) {
				err := repository.CreateResourceSchema(context.Background(), schema.ResourceRepresentation{
					ResourceType:     "host",
					ValidationSchema: validation.NewJsonSchemaValidatorFromString(validCommonSchema),
				})
				assert.NoError(t, err)
			},
			expectErr: false,
		},
		{
			name:                 "No common schema for host",
			resourceType:         "host",
			commonRepresentation: map[string]interface{}{"workspace_id": "ws-123"},
			setupRepository: func(repository schema.Repository) {
				err := repository.CreateResourceSchema(context.Background(), schema.ResourceRepresentation{
					ResourceType:     "host",
					ValidationSchema: nil,
				})
				assert.NoError(t, err)
			},
			expectErr:     true,
			expectedError: "no schema found for 'host'",
		},
		{
			name:                 "Empty common representation with schema",
			resourceType:         "host",
			commonRepresentation: map[string]interface{}{},
			setupRepository: func(repository schema.Repository) {
				err := repository.CreateResourceSchema(context.Background(), schema.ResourceRepresentation{
					ResourceType:     "host",
					ValidationSchema: validation.NewJsonSchemaValidatorFromString(validCommonSchema),
				})
				assert.NoError(t, err)
			},
			expectErr:     true,
			expectedError: "validation failed",
		},
		{
			name:         "Invalid common representation (wrong type)",
			resourceType: "host",
			commonRepresentation: map[string]interface{}{
				"workspace_id": 12345, // Should be string
			},
			setupRepository: func(repository schema.Repository) {
				err := repository.CreateResourceSchema(context.Background(), schema.ResourceRepresentation{
					ResourceType:     "host",
					ValidationSchema: validation.NewJsonSchemaValidatorFromString(validCommonSchema),
				})
				assert.NoError(t, err)
			},
			expectErr:     true,
			expectedError: "validation failed",
		},
		{
			name:                 "Resource does not exist",
			resourceType:         "invalid_resource",
			commonRepresentation: map[string]interface{}{"workspace_id": "ws-123"},
			setupRepository: func(repository schema.Repository) {
				// empty
			},
			expectErr:     true,
			expectedError: schema.ResourceSchemaNotFound.Error(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeRepo := data.NewInMemorySchemaRepository()
			tt.setupRepository(fakeRepo)
			sc := NewSchemaUsecase(data.NewInMemorySchemaRepository(), log.NewHelper(log.DefaultLogger))
			ctx := context.Background()

			err := sc.CommonShallowValidate(ctx, tt.resourceType, tt.commonRepresentation)

			if tt.expectErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestSchemaServiceImpl_ReporterShallowValidate(t *testing.T) {
	validReporterSchema := validation.NewJsonSchemaValidatorFromString(`{
		"$schema": "http://json-schema.org/draft-07/schema#",
		"type": "object",
		"properties": {
			"satellite_id": { "type": "string" }
		},
		"required": ["satellite_id"]
	}`)

	tests := []struct {
		name                   string
		resourceType           string
		reporterType           string
		reporterRepresentation map[string]interface{}
		setupRepository        func(repository schema.Repository)
		expectErr              bool
		expectedError          string
	}{
		{
			name:         "Valid reporter representation",
			resourceType: "host",
			reporterType: "hbi",
			reporterRepresentation: map[string]interface{}{
				"satellite_id": "sat-123",
			},
			setupRepository: func(repository schema.Repository) {
				err := repository.CreateResourceSchema(context.Background(), schema.ResourceRepresentation{
					ResourceType:     "host",
					ValidationSchema: nil,
				})
				assert.NoError(t, err)

				err = repository.CreateReporterSchema(context.Background(), schema.ReporterRepresentation{
					ResourceType:     "host",
					ReporterType:     "hbi",
					ValidationSchema: validReporterSchema,
				})
				assert.NoError(t, err)
			},
			expectErr: false,
		},
		{
			name:                   "No reporter schema but representation provided",
			resourceType:           "host",
			reporterType:           "hbi",
			reporterRepresentation: map[string]interface{}{"satellite_id": "sat-123"},
			setupRepository: func(repository schema.Repository) {
				err := repository.CreateResourceSchema(context.Background(), schema.ResourceRepresentation{
					ResourceType:     "host",
					ValidationSchema: nil,
				})
				assert.NoError(t, err)

				err = repository.CreateReporterSchema(context.Background(), schema.ReporterRepresentation{
					ResourceType:     "host",
					ReporterType:     "hbi",
					ValidationSchema: nil,
				})
				assert.NoError(t, err)
			},
			expectErr:     true,
			expectedError: "no schema found for 'host:hbi', but reporter representation was provided",
		},
		{
			name:                   "Empty reporter representation with schema",
			resourceType:           "host",
			reporterType:           "hbi",
			reporterRepresentation: map[string]interface{}{},
			setupRepository: func(repository schema.Repository) {
				err := repository.CreateResourceSchema(context.Background(), schema.ResourceRepresentation{
					ResourceType:     "host",
					ValidationSchema: nil,
				})
				assert.NoError(t, err)

				err = repository.CreateReporterSchema(context.Background(), schema.ReporterRepresentation{
					ResourceType:     "host",
					ReporterType:     "hbi",
					ValidationSchema: validReporterSchema,
				})
				assert.NoError(t, err)
			},
			expectErr:     true,
			expectedError: "validation failed",
		},
		{
			name:         "Invalid reporter representation (wrong type)",
			resourceType: "host",
			reporterType: "hbi",
			reporterRepresentation: map[string]interface{}{
				"satellite_id": 12345, // Should be string
			},
			setupRepository: func(repository schema.Repository) {
				err := repository.CreateResourceSchema(context.Background(), schema.ResourceRepresentation{
					ResourceType:     "host",
					ValidationSchema: nil,
				})
				assert.NoError(t, err)

				err = repository.CreateReporterSchema(context.Background(), schema.ReporterRepresentation{
					ResourceType:     "host",
					ReporterType:     "hbi",
					ValidationSchema: validReporterSchema,
				})
				assert.NoError(t, err)
			},
			expectErr:     true,
			expectedError: "validation failed",
		},
		{
			name:                   "Reporter does not exist",
			resourceType:           "host",
			reporterType:           "invalid_reporter",
			reporterRepresentation: map[string]interface{}{"satellite_id": "sat-123"},
			setupRepository: func(repository schema.Repository) {
				err := repository.CreateResourceSchema(context.Background(), schema.ResourceRepresentation{
					ResourceType:     "host",
					ValidationSchema: nil,
				})
				assert.NoError(t, err)
			},
			expectErr:     true,
			expectedError: schema.ReporterSchemaNotfound.Error(),
		},
		{
			name:                   "Resource does not exist",
			resourceType:           "some-resource",
			reporterType:           "some-reporter",
			reporterRepresentation: map[string]interface{}{"satellite_id": "sat-123"},
			setupRepository: func(repository schema.Repository) {
				// empty
			},
			expectErr:     true,
			expectedError: "resource type some-resource does not exist",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeRepo := data.NewInMemorySchemaRepository()
			tt.setupRepository(fakeRepo)
			sc := NewSchemaUsecase(data.NewInMemorySchemaRepository(), log.NewHelper(log.DefaultLogger))
			ctx := context.Background()

			err := sc.ReporterShallowValidate(ctx, tt.resourceType, tt.reporterType, tt.reporterRepresentation)

			if tt.expectErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
