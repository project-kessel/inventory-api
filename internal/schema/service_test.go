package schema

import (
	"context"
	"fmt"
	"testing"

	"github.com/project-kessel/inventory-api/internal/schema/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockSchemaRepository is a mock implementation of api.SchemaRepository
type MockSchemaRepository struct {
	mock.Mock
}

func (m *MockSchemaRepository) GetResources(ctx context.Context) ([]string, error) {
	args := m.Called(ctx)
	return args.Get(0).([]string), args.Error(1)
}

func (m *MockSchemaRepository) CreateResource(ctx context.Context, resource api.Resource) error {
	args := m.Called(ctx, resource)
	return args.Error(0)
}

func (m *MockSchemaRepository) GetResource(ctx context.Context, resourceType string) (api.Resource, error) {
	args := m.Called(ctx, resourceType)
	return args.Get(0).(api.Resource), args.Error(1)
}

func (m *MockSchemaRepository) UpdateResource(ctx context.Context, resource api.Resource) error {
	args := m.Called(ctx, resource)
	return args.Error(0)
}

func (m *MockSchemaRepository) DeleteResource(ctx context.Context, resourceType string) error {
	args := m.Called(ctx, resourceType)
	return args.Error(0)
}

func (m *MockSchemaRepository) GetResourceReporters(ctx context.Context, resourceType string) ([]string, error) {
	args := m.Called(ctx, resourceType)
	return args.Get(0).([]string), args.Error(1)
}

func (m *MockSchemaRepository) CreateResourceReporter(ctx context.Context, resourceReporter api.ResourceReporter) error {
	args := m.Called(ctx, resourceReporter)
	return args.Error(0)
}

func (m *MockSchemaRepository) GetResourceReporter(ctx context.Context, resourceType string, reporterType string) (api.ResourceReporter, error) {
	args := m.Called(ctx, resourceType, reporterType)
	return args.Get(0).(api.ResourceReporter), args.Error(1)
}

func (m *MockSchemaRepository) UpdateResourceReporter(ctx context.Context, resourceReporter api.ResourceReporter) error {
	args := m.Called(ctx, resourceReporter)
	return args.Error(0)
}

func (m *MockSchemaRepository) DeleteResourceReporter(ctx context.Context, resourceType string, reporterType string) error {
	args := m.Called(ctx, resourceType, reporterType)
	return args.Error(0)
}

func TestSchemaServiceImpl_ValidateReporterForResource(t *testing.T) {
	tests := []struct {
		name          string
		resourceType  string
		reporterType  string
		setupMock     func(*MockSchemaRepository)
		expectErr     bool
		expectedError string
	}{
		{
			name:         "Valid resource and reporter combination",
			resourceType: "host",
			reporterType: "hbi",
			setupMock: func(m *MockSchemaRepository) {
				m.On("GetResourceReporter", mock.Anything, "host", "hbi").Return(api.ResourceReporter{
					ResourceType:   "host",
					ReporterType:   "hbi",
					ReporterSchema: `{"type": "object"}`,
				}, nil)
			},
			expectErr: false,
		},
		{
			name:         "Invalid resource and reporter combination",
			resourceType: "host",
			reporterType: "invalid_reporter",
			setupMock: func(m *MockSchemaRepository) {
				m.On("GetResourceReporter", mock.Anything, "host", "invalid_reporter").Return(api.ResourceReporter{}, fmt.Errorf("reporter invalid_reporter does not exist"))
			},
			expectErr:     true,
			expectedError: "reporter invalid_reporter does not exist",
		},
		{
			name:         "Resource type does not exist",
			resourceType: "invalid_resource",
			reporterType: "hbi",
			setupMock: func(m *MockSchemaRepository) {
				m.On("GetResourceReporter", mock.Anything, "invalid_resource", "hbi").Return(api.ResourceReporter{}, fmt.Errorf("resource type invalid_resource does not exist"))
			},
			expectErr:     true,
			expectedError: "resource type invalid_resource does not exist",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := new(MockSchemaRepository)
			tt.setupMock(mockRepo)

			service := NewSchemaService(mockRepo)
			ctx := context.Background()

			err := service.ValidateReporterForResource(ctx, tt.resourceType, tt.reporterType)

			if tt.expectErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
			} else {
				assert.NoError(t, err)
			}

			mockRepo.AssertExpectations(t)
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
		setupMock            func(*MockSchemaRepository)
		expectErr            bool
		expectedError        string
	}{
		{
			name:         "Valid common representation",
			resourceType: "host",
			commonRepresentation: map[string]interface{}{
				"workspace_id": "ws-123",
			},
			setupMock: func(m *MockSchemaRepository) {
				m.On("GetResource", mock.Anything, "host").Return(api.Resource{
					ResourceType: "host",
					CommonSchema: validCommonSchema,
				}, nil)
			},
			expectErr: false,
		},
		{
			name:                 "No common schema for host",
			resourceType:         "host",
			commonRepresentation: map[string]interface{}{"workspace_id": "ws-123"},
			setupMock: func(m *MockSchemaRepository) {
				m.On("GetResource", mock.Anything, "host").Return(api.Resource{
					ResourceType: "host",
					CommonSchema: "",
				}, nil)
			},
			expectErr:     true,
			expectedError: "no schema found for 'host'",
		},
		{
			name:                 "Empty common representation with schema",
			resourceType:         "host",
			commonRepresentation: map[string]interface{}{},
			setupMock: func(m *MockSchemaRepository) {
				m.On("GetResource", mock.Anything, "host").Return(api.Resource{
					ResourceType: "host",
					CommonSchema: validCommonSchema,
				}, nil)
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
			setupMock: func(m *MockSchemaRepository) {
				m.On("GetResource", mock.Anything, "host").Return(api.Resource{
					ResourceType: "host",
					CommonSchema: validCommonSchema,
				}, nil)
			},
			expectErr:     true,
			expectedError: "validation failed",
		},
		{
			name:                 "Resource does not exist",
			resourceType:         "invalid_resource",
			commonRepresentation: map[string]interface{}{"workspace_id": "ws-123"},
			setupMock: func(m *MockSchemaRepository) {
				m.On("GetResource", mock.Anything, "invalid_resource").Return(api.Resource{}, fmt.Errorf("resource invalid_resource does not exist"))
			},
			expectErr:     true,
			expectedError: "resource invalid_resource does not exist",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := new(MockSchemaRepository)
			tt.setupMock(mockRepo)

			service := NewSchemaService(mockRepo)
			ctx := context.Background()

			err := service.CommonShallowValidate(ctx, tt.resourceType, tt.commonRepresentation)

			if tt.expectErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
			} else {
				assert.NoError(t, err)
			}

			mockRepo.AssertExpectations(t)
		})
	}
}

func TestSchemaServiceImpl_ReporterShallowValidate(t *testing.T) {
	validReporterSchema := `{
		"$schema": "http://json-schema.org/draft-07/schema#",
		"type": "object",
		"properties": {
			"satellite_id": { "type": "string" }
		},
		"required": ["satellite_id"]
	}`

	tests := []struct {
		name                   string
		resourceType           string
		reporterType           string
		reporterRepresentation map[string]interface{}
		setupMock              func(*MockSchemaRepository)
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
			setupMock: func(m *MockSchemaRepository) {
				m.On("GetResourceReporter", mock.Anything, "host", "hbi").Return(api.ResourceReporter{
					ResourceType:   "host",
					ReporterType:   "hbi",
					ReporterSchema: validReporterSchema,
				}, nil)
			},
			expectErr: false,
		},
		{
			name:                   "No reporter schema but representation provided",
			resourceType:           "host",
			reporterType:           "hbi",
			reporterRepresentation: map[string]interface{}{"satellite_id": "sat-123"},
			setupMock: func(m *MockSchemaRepository) {
				m.On("GetResourceReporter", mock.Anything, "host", "hbi").Return(api.ResourceReporter{
					ResourceType:   "host",
					ReporterType:   "hbi",
					ReporterSchema: "",
				}, nil)
			},
			expectErr:     true,
			expectedError: "no schema found for 'host:hbi', but reporter representation was provided",
		},
		{
			name:                   "Empty reporter representation with schema",
			resourceType:           "host",
			reporterType:           "hbi",
			reporterRepresentation: map[string]interface{}{},
			setupMock: func(m *MockSchemaRepository) {
				m.On("GetResourceReporter", mock.Anything, "host", "hbi").Return(api.ResourceReporter{
					ResourceType:   "host",
					ReporterType:   "hbi",
					ReporterSchema: validReporterSchema,
				}, nil)
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
			setupMock: func(m *MockSchemaRepository) {
				m.On("GetResourceReporter", mock.Anything, "host", "hbi").Return(api.ResourceReporter{
					ResourceType:   "host",
					ReporterType:   "hbi",
					ReporterSchema: validReporterSchema,
				}, nil)
			},
			expectErr:     true,
			expectedError: "validation failed",
		},
		{
			name:                   "Reporter does not exist",
			resourceType:           "host",
			reporterType:           "invalid_reporter",
			reporterRepresentation: map[string]interface{}{"satellite_id": "sat-123"},
			setupMock: func(m *MockSchemaRepository) {
				m.On("GetResourceReporter", mock.Anything, "host", "invalid_reporter").Return(api.ResourceReporter{}, fmt.Errorf("reporter invalid_reporter does not exist"))
			},
			expectErr:     true,
			expectedError: "reporter invalid_reporter does not exist",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := new(MockSchemaRepository)
			tt.setupMock(mockRepo)

			service := NewSchemaService(mockRepo)
			ctx := context.Background()

			err := service.ReporterShallowValidate(ctx, tt.resourceType, tt.reporterType, tt.reporterRepresentation)

			if tt.expectErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
			} else {
				assert.NoError(t, err)
			}

			mockRepo.AssertExpectations(t)
		})
	}
}
