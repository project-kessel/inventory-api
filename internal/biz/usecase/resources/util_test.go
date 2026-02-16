package resources

import (
	"context"
	"testing"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/project-kessel/inventory-api/internal/biz/model"
	"github.com/project-kessel/inventory-api/internal/biz/schema"
	"github.com/project-kessel/inventory-api/internal/biz/schema/validation"
	"github.com/project-kessel/inventory-api/internal/data"
	"github.com/stretchr/testify/assert"
)

func TestRemoveNulls(t *testing.T) {
	tests := []struct {
		name     string
		input    map[string]interface{}
		expected map[string]interface{}
	}{
		{
			name: "HBI host with all fields",
			input: map[string]interface{}{
				"insights_id":  "b5c36330-79cf-426e-a950-df2e972c3ef4",
				"satellite_id": "some-satellite-id",
				"ansible_host": "my-ansible-host.example.com",
			},
			expected: map[string]interface{}{
				"insights_id":  "b5c36330-79cf-426e-a950-df2e972c3ef4",
				"satellite_id": "some-satellite-id",
				"ansible_host": "my-ansible-host.example.com",
			},
		},
		{
			name: "HBI host with null ansible_host",
			input: map[string]interface{}{
				"insights_id":  "b5c36330-79cf-426e-a950-df2e972c3ef4",
				"satellite_id": "some-satellite-id",
				"ansible_host": nil,
			},
			expected: map[string]interface{}{
				"insights_id":  "b5c36330-79cf-426e-a950-df2e972c3ef4",
				"satellite_id": "some-satellite-id",
			},
		},
		{
			name: "HBI host with multiple nulls",
			input: map[string]interface{}{
				"insights_id":  "b5c36330-79cf-426e-a950-df2e972c3ef4",
				"satellite_id": nil,
				"ansible_host": "null",
			},
			expected: map[string]interface{}{
				"insights_id": "b5c36330-79cf-426e-a950-df2e972c3ef4",
			},
		},
		{
			name: "nested nulls in a generic structure",
			input: map[string]interface{}{
				"metadata": map[string]interface{}{
					"source": "some-source",
					"notes":  nil,
				},
				"data": "some-data",
			},
			expected: map[string]interface{}{
				"metadata": map[string]interface{}{
					"source": "some-source",
				},
				"data": "some-data",
			},
		},
		{
			name:     "empty map",
			input:    map[string]interface{}{},
			expected: map[string]interface{}{},
		},
		{
			name: "nested string 'null' value",
			input: map[string]interface{}{
				"details": map[string]interface{}{
					"comment": "NULL",
					"user":    "alice",
				},
			},
			expected: map[string]interface{}{
				"details": map[string]interface{}{
					"user": "alice",
				},
			},
		},
		{
			name: "nested string 'null' value case insensitive",
			input: map[string]interface{}{
				"details": map[string]interface{}{
					"comment": "null",
					"user":    "alice",
				},
			},
			expected: map[string]interface{}{
				"details": map[string]interface{}{
					"user": "alice",
				},
			},
		},
		{
			name: "nested map becomes empty",
			input: map[string]interface{}{
				"meta": map[string]interface{}{
					"comment": nil,
				},
			},
			expected: map[string]interface{}{},
		},
		{
			name: "deeply nested null values",
			input: map[string]interface{}{
				"a": map[string]interface{}{
					"b": map[string]interface{}{
						"c": nil,
						"d": "valid",
					},
				},
			},
			expected: map[string]interface{}{
				"a": map[string]interface{}{
					"b": map[string]interface{}{
						"d": "valid",
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := removeNulls(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// Helper to create a usecase with schema repository for validation tests
func newValidationTestUsecase(t *testing.T, schemaRepository schema.Repository) *Usecase {
	return New(
		data.NewFakeResourceRepository(),
		schemaRepository,
		nil, // authz not needed for validation
		"test-topic",
		log.DefaultLogger,
		nil, nil,
		&UsecaseConfig{},
		nil, nil, nil,
	)
}

func TestValidateReportResourceCommand_Success(t *testing.T) {
	ctx := context.Background()
	schemaRepository := data.NewInMemorySchemaRepository()

	err := schemaRepository.CreateResourceSchema(ctx, schema.ResourceRepresentation{
		ResourceType: "host",
		ValidationSchema: validation.NewJsonSchemaValidatorFromString(`{
		  "$schema": "http://json-schema.org/draft-07/schema#",
		  "type": "object",
		  "properties": {
			"workspace_id": { "type": "string" }
		  },
		  "required": ["workspace_id"]
		}`),
	})
	assert.NoError(t, err)

	err = schemaRepository.CreateReporterSchema(ctx, schema.ReporterRepresentation{
		ResourceType: "host",
		ReporterType: "hbi",
		ValidationSchema: validation.NewJsonSchemaValidatorFromString(`{
		  "$schema": "http://json-schema.org/draft-07/schema#",
		  "type": "object",
		  "properties": {
			"satellite_id": { "type": "string" },
			"ansible_host": { "type": "string" }
		  },
		  "required": []
		}`),
	})
	assert.NoError(t, err)

	usecase := newValidationTestUsecase(t, schemaRepository)
	cmd := fixture().WithData("host", "hbi", "instance-1", "host-1",
		map[string]interface{}{
			"satellite_id": "2c4196f1-0371-4f4c-8913-e113cfaa6e67",
			"ansible_host": "host-1",
		},
		map[string]interface{}{
			"workspace_id": "ws-123",
		},
	)

	err = usecase.validateReportResourceCommand(ctx, cmd)
	assert.NoError(t, err)
}

func TestValidateReportResourceCommand_FieldExtractionErrors(t *testing.T) {
	ctx := context.Background()
	schemaRepository := data.NewInMemorySchemaRepository()

	err := schemaRepository.CreateResourceSchema(ctx, schema.ResourceRepresentation{
		ResourceType: "host",
		ValidationSchema: validation.NewJsonSchemaValidatorFromString(`{
		  "$schema": "http://json-schema.org/draft-07/schema#",
		  "type": "object",
		  "properties": {
			"workspace_id": { "type": "string" }
		  },
		  "required": ["workspace_id"]
		}`),
	})
	assert.NoError(t, err)

	err = schemaRepository.CreateReporterSchema(ctx, schema.ReporterRepresentation{
		ResourceType: "host",
		ReporterType: "hbi",
		ValidationSchema: validation.NewJsonSchemaValidatorFromString(`{
		  "$schema": "http://json-schema.org/draft-07/schema#",
		  "type": "object",
		  "properties": {
			"satellite_id": { "type": "string" }
		  },
		  "required": ["satellite_id"]
		}`),
	})
	assert.NoError(t, err)

	usecase := newValidationTestUsecase(t, schemaRepository)

	tests := []struct {
		name   string
		cmd    ReportResourceCommand
		expect string
	}{
		{
			name: "reporter type not allowed for resource",
			cmd: fixture().WithData("host", "unknown_reporter", "instance-1", "host-1",
				map[string]interface{}{"key": "value"},
				map[string]interface{}{"workspace_id": "ws-123"},
			),
			expect: "reporter unknown_reporter does not report resource types: host",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := usecase.validateReportResourceCommand(ctx, tc.cmd)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), tc.expect)
		})
	}
}

func TestValidateReportResourceCommand_SchemaBasedValidation(t *testing.T) {
	tests := []struct {
		name           string
		resourceType   string
		reporterType   string
		reporterData   map[string]interface{}
		commonData     map[string]interface{}
		reporterSchema string
		commonSchema   string
		expectError    bool
		expectedError  string
	}{
		{
			name:         "Reporter schema with NO required fields - empty reporter data should pass",
			resourceType: "k8s_policy",
			reporterType: "acm",
			reporterData: map[string]interface{}{"optional_field": "value"},
			commonData:   map[string]interface{}{"workspace_id": "ws-456"},
			reporterSchema: `{
				"$schema": "http://json-schema.org/draft-07/schema#",
				"type": "object",
				"properties": {
					"policy_id": { "type": "string" }
				},
				"required": []
			}`,
			commonSchema: `{
				"$schema": "http://json-schema.org/draft-07/schema#",
				"type": "object",
				"properties": {
					"workspace_id": { "type": "string" }
				},
				"required": ["workspace_id"]
			}`,
			expectError: false,
		},
		{
			name:         "Common schema with NO required fields - minimal common data should pass",
			resourceType: "k8s_policy",
			reporterType: "acm",
			reporterData: map[string]interface{}{"policy_id": "pol-123"},
			commonData:   map[string]interface{}{"optional": "value"},
			reporterSchema: `{
				"$schema": "http://json-schema.org/draft-07/schema#",
				"type": "object",
				"properties": {
					"policy_id": { "type": "string" }
				},
				"required": []
			}`,
			commonSchema: `{
				"$schema": "http://json-schema.org/draft-07/schema#",
				"type": "object",
				"properties": {
					"workspace_id": { "type": "string" }
				},
				"required": []
			}`,
			expectError: false,
		},
		{
			name:         "Common schema with required fields - missing required field should fail",
			resourceType: "k8s_policy",
			reporterType: "acm",
			reporterData: map[string]interface{}{"policy_id": "pol-123"},
			commonData:   map[string]interface{}{"other_field": "value"},
			reporterSchema: `{
				"$schema": "http://json-schema.org/draft-07/schema#",
				"type": "object",
				"properties": {
					"policy_id": { "type": "string" }
				},
				"required": []
			}`,
			commonSchema: `{
				"$schema": "http://json-schema.org/draft-07/schema#",
				"type": "object",
				"properties": {
					"workspace_id": { "type": "string" }
				},
				"required": ["workspace_id"]
			}`,
			expectError:   true,
			expectedError: "workspace_id",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			schemaRepository := data.NewInMemorySchemaRepository()

			err := schemaRepository.CreateResourceSchema(ctx, schema.ResourceRepresentation{
				ResourceType:     tc.resourceType,
				ValidationSchema: validation.NewJsonSchemaValidatorFromString(tc.commonSchema),
			})
			assert.NoError(t, err)

			err = schemaRepository.CreateReporterSchema(ctx, schema.ReporterRepresentation{
				ResourceType:     tc.resourceType,
				ReporterType:     tc.reporterType,
				ValidationSchema: validation.NewJsonSchemaValidatorFromString(tc.reporterSchema),
			})
			assert.NoError(t, err)

			usecase := newValidationTestUsecase(t, schemaRepository)

			localResId, _ := model.NewLocalResourceId("test-resource")
			resType, _ := model.NewResourceType(tc.resourceType)
			repType, _ := model.NewReporterType(tc.reporterType)
			repInstanceId, _ := model.NewReporterInstanceId("test-instance")
			apiHref, _ := model.NewApiHref("https://api.example.com")
			consoleHref, _ := model.NewConsoleHref("https://console.example.com")
			reporterRep, _ := model.NewRepresentation(tc.reporterData)
			commonRep, _ := model.NewRepresentation(tc.commonData)

			var reporterRepPtr *model.Representation
			if reporterRep != nil {
				reporterRepPtr = &reporterRep
			}
			var commonRepPtr *model.Representation
			if commonRep != nil {
				commonRepPtr = &commonRep
			}

			cmd := ReportResourceCommand{
				LocalResourceId:        localResId,
				ResourceType:           resType,
				ReporterType:           repType,
				ReporterInstanceId:     repInstanceId,
				ApiHref:                apiHref,
				ConsoleHref:            &consoleHref,
				ReporterRepresentation: reporterRepPtr,
				CommonRepresentation:   commonRepPtr,
				WriteVisibility:        WriteVisibilityMinimizeLatency,
			}

			err = usecase.validateReportResourceCommand(ctx, cmd)
			if tc.expectError {
				assert.Error(t, err)
				if tc.expectedError != "" {
					assert.Contains(t, err.Error(), tc.expectedError)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
