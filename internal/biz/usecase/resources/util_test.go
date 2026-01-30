package resources

import (
	"context"
	"testing"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/project-kessel/inventory-api/internal/biz/schema"
	"github.com/project-kessel/inventory-api/internal/biz/schema/validation"
	"github.com/project-kessel/inventory-api/internal/data"
	"google.golang.org/protobuf/types/known/structpb"

	pbv1beta2 "github.com/project-kessel/inventory-api/api/kessel/inventory/v1beta2"
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

func TestValidateReportResourceJSON_Success(t *testing.T) {
	reporterStruct, err := structpb.NewStruct(map[string]interface{}{
		"satellite_id":            "2c4196f1-0371-4f4c-8913-e113cfaa6e67",
		"subscription_manager_id": "af94f92b-0b65-4cac-b449-6b77e665a08f",
		"insights_id":             "05707922-7b0a-4fe6-982d-6adbc7695b8f",
		"ansible_host":            "host-1",
	})
	assert.NoError(t, err)

	commonStruct, err := structpb.NewStruct(map[string]interface{}{
		"workspace_id": "12",
	})
	assert.NoError(t, err)
	msg := &pbv1beta2.ReportResourceRequest{
		Type:         "host",
		ReporterType: "hbi",
		Representations: &pbv1beta2.ResourceRepresentations{
			Metadata: &pbv1beta2.RepresentationMetadata{
				LocalResourceId: "id",
				ApiHref:         "url",
			},
			Reporter: reporterStruct,
			Common:   commonStruct,
		},
	}

	ctx := context.Background()
	schemaRepository := data.NewInMemorySchemaRepository()

	err = schemaRepository.CreateResourceSchema(ctx, schema.ResourceRepresentation{
		ResourceType: "host",
		ValidationSchema: validation.NewJsonSchemaValidatorFromString(`{
		  "$schema": "http://json-schema.org/draft-07/schema#",
		  "type": "object",
		  "properties": {
			"workspace_id": { "type": "string" }
		  },
		  "required": [
			"workspace_id"
		  ]
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
			"satellite_id": { "type": "string", "format": "uuid" },
			"subscription_manager_id": { "type": "string", "format": "uuid" },
			"insights_id": { "type": "string", "format": "uuid" },
			"ansible_host": { "type": "string", "maxLength": 255 }
		  },
		  "required": []
		}`),
	})
	assert.NoError(t, err)

	err = validateReportResourceRequest(ctx, msg, NewSchemaUsecase(schemaRepository, log.NewHelper(log.DefaultLogger)))
	assert.NoError(t, err)
}

func AsStruct(t *testing.T, m map[string]interface{}) *structpb.Struct {
	s, err := structpb.NewStruct(m)
	assert.NoError(t, err)
	return s
}

func TestValidateReportResourceJSON_FieldExtractionErrors(t *testing.T) {

	// Good base case
	baseMsg := &pbv1beta2.ReportResourceRequest{
		Type:               "host",
		ReporterType:       "hbi",
		ReporterInstanceId: "2c4196f1-0371-4f4c-8913-e113cfaa6e68",
		Representations: &pbv1beta2.ResourceRepresentations{
			Metadata: &pbv1beta2.RepresentationMetadata{
				LocalResourceId: "id",
				ApiHref:         "url",
			},
			Reporter: AsStruct(t, map[string]interface{}{
				"satellite_id":            "2c4196f1-0371-4f4c-8913-e113cfaa6e67",
				"subscription_manager_id": "af94f92b-0b65-4cac-b449-6b77e665a08f",
				"insights_id":             "05707922-7b0a-4fe6-982d-6adbc7695b8f",
				"ansible_host":            "host-1",
			}),
			Common: AsStruct(t, map[string]interface{}{
				"workspace_id": "a64d17d0-aec3-410a-acd0-e0b85b22c076",
			}),
		},
	}

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
		  "required": [
			"workspace_id"
		  ]
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
			"satellite_id": { "type": "string", "format": "uuid" },
			"subscription_manager_id": { "type": "string", "format": "uuid" },
			"insights_id": { "type": "string", "format": "uuid" },
			"ansible_host": { "type": "string", "maxLength": 255 }
		  },
		  "required": [
			"subscription_manager_id"
			]
		}`),
	})
	assert.NoError(t, err)

	tests := []struct {
		name   string
		msg    *pbv1beta2.ReportResourceRequest
		expect string // part of expected error
	}{
		{
			name: "missing type",
			msg: &pbv1beta2.ReportResourceRequest{
				ReporterType:    "hbi",
				Representations: baseMsg.Representations,
			},
			expect: "missing 'type'",
		},
		{
			name: "missing reporterType",
			msg: &pbv1beta2.ReportResourceRequest{
				Type:            "host",
				Representations: baseMsg.Representations,
			},
			expect: "missing 'reporterType'",
		},
		{
			name: "missing representations",
			msg: &pbv1beta2.ReportResourceRequest{
				Type:            "host",
				ReporterType:    "hbi",
				Representations: nil,
			},
			expect: "missing 'representations'",
		},
		{
			name: "missing reporter in representations",
			msg: &pbv1beta2.ReportResourceRequest{
				Type:         "host",
				ReporterType: "hbi",
				Representations: &pbv1beta2.ResourceRepresentations{
					Metadata: baseMsg.Representations.Metadata,
					Common:   baseMsg.Representations.Common,
					// No Reporter
				},
			},
			expect: "missing 'reporter'",
		},
		{
			name: "missing common in representations",
			msg: &pbv1beta2.ReportResourceRequest{
				Type:         "host",
				ReporterType: "hbi",
				Representations: &pbv1beta2.ResourceRepresentations{
					Metadata: baseMsg.Representations.Metadata,
					Reporter: baseMsg.Representations.Reporter,
					// No Common
				},
			},
			expect: "missing 'common'",
		},
		{
			name: "missing common in representations",
			msg: &pbv1beta2.ReportResourceRequest{
				Type:         "host",
				ReporterType: "bar",
				Representations: &pbv1beta2.ResourceRepresentations{
					Metadata: baseMsg.Representations.Metadata,
					Reporter: baseMsg.Representations.Reporter,
					Common:   baseMsg.Representations.Common,
				},
			},
			expect: "reporter bar does not report resource types: host",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := validateReportResourceRequest(ctx, tc.msg, NewSchemaUsecase(schemaRepository, log.NewHelper(log.DefaultLogger)))
			assert.Error(t, err)
			assert.Contains(t, err.Error(), tc.expect)
		})
	}
}

func TestValidateReportResourceJSON_SchemaBasedValidation(t *testing.T) {
	tests := []struct {
		name           string
		msg            *pbv1beta2.ReportResourceRequest
		reporterSchema string
		commonSchema   string
		expectError    bool
		expectedError  string
	}{
		{
			name: "Reporter schema with NO required fields - missing reporter data should pass",
			msg: &pbv1beta2.ReportResourceRequest{
				Type:         "k8s_policy",
				ReporterType: "acm",
				Representations: &pbv1beta2.ResourceRepresentations{
					Metadata: &pbv1beta2.RepresentationMetadata{
						LocalResourceId: "policy-123",
						ApiHref:         "url",
					},
					// No Reporter field
					Common: AsStruct(t, map[string]interface{}{
						"workspace_id": "ws-456",
					}),
				},
			},
			reporterSchema: `{
				"$schema": "http://json-schema.org/draft-07/schema#",
				"type": "object",
				"properties": {
					"policy_id": { "type": "string" },
					"policy_name": { "type": "string" }
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
			name: "Common schema with NO required fields - missing common data should pass",
			msg: &pbv1beta2.ReportResourceRequest{
				Type:         "k8s_policy",
				ReporterType: "acm",
				Representations: &pbv1beta2.ResourceRepresentations{
					Metadata: &pbv1beta2.RepresentationMetadata{
						LocalResourceId: "policy-123",
						ApiHref:         "url",
					},
					Reporter: AsStruct(t, map[string]interface{}{
						"policy_id":   "pol-abc123",
						"policy_name": "security-policy",
					}),
					// No Common field
				},
			},
			reporterSchema: `{
				"$schema": "http://json-schema.org/draft-07/schema#",
				"type": "object",
				"properties": {
					"policy_id": { "type": "string" },
					"policy_name": { "type": "string" }
				},
				"required": []
			}`,
			commonSchema: `{
				"$schema": "http://json-schema.org/draft-07/schema#",
				"type": "object",
				"properties": {
					"workspace_id": { "type": "string" },
					"organization_id": { "type": "string" }
				},
				"required": []
			}`,
			expectError: false,
		},
		{
			name: "Both schemas with NO required fields - missing both reporter and common should pass",
			msg: &pbv1beta2.ReportResourceRequest{
				Type:         "k8s_policy",
				ReporterType: "acm",
				Representations: &pbv1beta2.ResourceRepresentations{
					Metadata: &pbv1beta2.RepresentationMetadata{
						LocalResourceId: "policy-123",
						ApiHref:         "url",
					},
					// No Reporter or Common fields
				},
			},
			reporterSchema: `{
				"$schema": "http://json-schema.org/draft-07/schema#",
				"type": "object",
				"properties": {
					"policy_id": { "type": "string" },
					"policy_name": { "type": "string" }
				}
			}`,
			commonSchema: `{
				"$schema": "http://json-schema.org/draft-07/schema#",
				"type": "object",
				"properties": {
					"workspace_id": { "type": "string" },
					"organization_id": { "type": "string" }
				}
			}`,
			expectError: false,
		},
		{
			name: "Reporter schema with required fields - missing reporter data should fail",
			msg: &pbv1beta2.ReportResourceRequest{
				Type:         "k8s_policy",
				ReporterType: "acm",
				Representations: &pbv1beta2.ResourceRepresentations{
					Metadata: &pbv1beta2.RepresentationMetadata{
						LocalResourceId: "policy-123",
						ApiHref:         "url",
					},
					// No Reporter field
					Common: AsStruct(t, map[string]interface{}{
						"workspace_id": "ws-456",
					}),
				},
			},
			reporterSchema: `{
				"$schema": "http://json-schema.org/draft-07/schema#",
				"type": "object",
				"properties": {
					"policy_id": { "type": "string" },
					"policy_name": { "type": "string" }
				},
				"required": ["policy_id"]
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
			expectedError: "missing 'reporter' field in payload - schema for 'k8s_policy:acm' has required fields",
		},
		{
			name: "Common schema with required fields - missing common data should fail",
			msg: &pbv1beta2.ReportResourceRequest{
				Type:         "k8s_policy",
				ReporterType: "acm",
				Representations: &pbv1beta2.ResourceRepresentations{
					Metadata: &pbv1beta2.RepresentationMetadata{
						LocalResourceId: "policy-123",
						ApiHref:         "url",
					},
					Reporter: AsStruct(t, map[string]interface{}{
						"policy_id":   "pol-abc123",
						"policy_name": "security-policy",
					}),
					// No Common field
				},
			},
			reporterSchema: `{
				"$schema": "http://json-schema.org/draft-07/schema#",
				"type": "object",
				"properties": {
					"policy_id": { "type": "string" },
					"policy_name": { "type": "string" }
				},
				"required": []
			}`,
			commonSchema: `{
				"$schema": "http://json-schema.org/draft-07/schema#",
				"type": "object",
				"properties": {
					"workspace_id": { "type": "string" },
					"organization_id": { "type": "string" }
				},
				"required": ["workspace_id"]
			}`,
			expectError:   true,
			expectedError: "missing 'common' field in payload - schema for 'k8s_policy' has required fields",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Setup schemas in cache
			ctx := context.Background()
			schemaRepository := data.NewInMemorySchemaRepository()

			// Setup config for k8s_policy with acm reporter
			err := schemaRepository.CreateResourceSchema(ctx, schema.ResourceRepresentation{
				ResourceType:     "k8s_policy",
				ValidationSchema: validation.NewJsonSchemaValidatorFromString(tc.commonSchema),
			})
			assert.NoError(t, err)

			err = schemaRepository.CreateReporterSchema(
				ctx,
				schema.ReporterRepresentation{
					ResourceType:     "k8s_policy",
					ReporterType:     "acm",
					ValidationSchema: validation.NewJsonSchemaValidatorFromString(tc.reporterSchema),
				},
			)
			assert.NoError(t, err)

			// Test the function
			err = validateReportResourceRequest(ctx, tc.msg, NewSchemaUsecase(schemaRepository, log.NewHelper(log.DefaultLogger)))
			if tc.expectError {
				assert.Error(t, err)
			}

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
