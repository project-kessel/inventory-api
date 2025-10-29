package schema

import (
	"context"
	"testing"

	"github.com/project-kessel/inventory-api/internal/schema/validation"

	"github.com/project-kessel/inventory-api/internal/schema/in_memory"

	"github.com/project-kessel/inventory-api/internal/schema/api"
	"github.com/stretchr/testify/assert"
)

func TestSchemaServiceImpl_ValidateReporterForResource(t *testing.T) {
	tests := []struct {
		name            string
		resourceType    string
		reporterType    string
		setupRepository func(repository api.SchemaRepository)
		isReporter      bool
		expectErr       bool
		expectedError   string
	}{
		{
			name:         "Valid resource and reporter combination",
			resourceType: "host",
			reporterType: "hbi",
			setupRepository: func(repository api.SchemaRepository) {
				err := repository.CreateResourceSchema(context.Background(), api.ResourceRepresentation{
					ResourceType:     "host",
					ValidationSchema: validation.NewJsonSchemaValidatorFromString(`{"type": "object"}`),
				})

				assert.NoError(t, err)

				err = repository.CreateReporterSchema(context.Background(), api.ReporterRepresentation{
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
			setupRepository: func(repository api.SchemaRepository) {
				err := repository.CreateResourceSchema(context.Background(), api.ResourceRepresentation{
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
			setupRepository: func(repository api.SchemaRepository) {
				// nothing here
			},
			isReporter:    false,
			expectErr:     true,
			expectedError: "resource type invalid_resource does not exist",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeRepo := in_memory.New()
			tt.setupRepository(fakeRepo)
			service := NewSchemaService(fakeRepo)
			ctx := context.Background()

			isReporter, err := service.IsReporterForResource(ctx, tt.resourceType, tt.reporterType)

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
		setupRepository      func(repository api.SchemaRepository)
		expectErr            bool
		expectedError        string
	}{
		{
			name:         "Valid common representation",
			resourceType: "host",
			commonRepresentation: map[string]interface{}{
				"workspace_id": "ws-123",
			},
			setupRepository: func(repository api.SchemaRepository) {
				err := repository.CreateResourceSchema(context.Background(), api.ResourceRepresentation{
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
			setupRepository: func(repository api.SchemaRepository) {
				err := repository.CreateResourceSchema(context.Background(), api.ResourceRepresentation{
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
			setupRepository: func(repository api.SchemaRepository) {
				err := repository.CreateResourceSchema(context.Background(), api.ResourceRepresentation{
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
			setupRepository: func(repository api.SchemaRepository) {
				err := repository.CreateResourceSchema(context.Background(), api.ResourceRepresentation{
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
			setupRepository: func(repository api.SchemaRepository) {
				// empty
			},
			expectErr:     true,
			expectedError: api.ResourceSchemaNotFound.Error(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeRepo := in_memory.New()
			tt.setupRepository(fakeRepo)

			service := NewSchemaService(fakeRepo)
			ctx := context.Background()

			err := service.CommonShallowValidate(ctx, tt.resourceType, tt.commonRepresentation)

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
		setupRepository        func(repository api.SchemaRepository)
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
			setupRepository: func(repository api.SchemaRepository) {
				err := repository.CreateResourceSchema(context.Background(), api.ResourceRepresentation{
					ResourceType:     "host",
					ValidationSchema: nil,
				})
				assert.NoError(t, err)

				err = repository.CreateReporterSchema(context.Background(), api.ReporterRepresentation{
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
			setupRepository: func(repository api.SchemaRepository) {
				err := repository.CreateResourceSchema(context.Background(), api.ResourceRepresentation{
					ResourceType:     "host",
					ValidationSchema: nil,
				})
				assert.NoError(t, err)

				err = repository.CreateReporterSchema(context.Background(), api.ReporterRepresentation{
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
			setupRepository: func(repository api.SchemaRepository) {
				err := repository.CreateResourceSchema(context.Background(), api.ResourceRepresentation{
					ResourceType:     "host",
					ValidationSchema: nil,
				})
				assert.NoError(t, err)

				err = repository.CreateReporterSchema(context.Background(), api.ReporterRepresentation{
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
			setupRepository: func(repository api.SchemaRepository) {
				err := repository.CreateResourceSchema(context.Background(), api.ResourceRepresentation{
					ResourceType:     "host",
					ValidationSchema: nil,
				})
				assert.NoError(t, err)

				err = repository.CreateReporterSchema(context.Background(), api.ReporterRepresentation{
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
			setupRepository: func(repository api.SchemaRepository) {
				err := repository.CreateResourceSchema(context.Background(), api.ResourceRepresentation{
					ResourceType:     "host",
					ValidationSchema: nil,
				})
				assert.NoError(t, err)
			},
			expectErr:     true,
			expectedError: api.ReporterSchemaNotfound.Error(),
		},
		{
			name:                   "Resource does not exist",
			resourceType:           "some-resource",
			reporterType:           "some-reporter",
			reporterRepresentation: map[string]interface{}{"satellite_id": "sat-123"},
			setupRepository: func(repository api.SchemaRepository) {
				// empty
			},
			expectErr:     true,
			expectedError: "resource type some-resource does not exist",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeRepo := in_memory.New()
			tt.setupRepository(fakeRepo)

			service := NewSchemaService(fakeRepo)
			ctx := context.Background()

			err := service.ReporterShallowValidate(ctx, tt.resourceType, tt.reporterType, tt.reporterRepresentation)

			if tt.expectErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
