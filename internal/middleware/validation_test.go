package middleware_test

import (
	"testing"

	"github.com/spf13/viper"

	"google.golang.org/protobuf/types/known/structpb"

	pbv1beta2 "github.com/project-kessel/inventory-api/api/kessel/inventory/v1beta2"

	"github.com/stretchr/testify/assert"

	"github.com/project-kessel/inventory-api/internal/middleware"
)

func AsStruct(t *testing.T, m map[string]interface{}) *structpb.Struct {
	s, err := structpb.NewStruct(m)
	assert.NoError(t, err)
	return s
}

func TestValidateReportResourceJSON_Success(t *testing.T) {
	reporterStruct, err := structpb.NewStruct(map[string]interface{}{
		"satellite_id":            "2c4196f1-0371-4f4c-8913-e113cfaa6e67",
		"subscription_manager_id": "af94f92b-0b65-4cac-b449-6b77e665a08f",
		"insights_inventory_id":   "05707922-7b0a-4fe6-982d-6adbc7695b8f",
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

	viper.Set("resources.use_cache", true)
	middleware.SchemaCache.Store("config:host", map[string]interface{}{
		"resource_reporters": []string{"hbi"},
	})

	middleware.SchemaCache.Store("host:hbi", `{
	  "$schema": "http://json-schema.org/draft-07/schema#",
	  "type": "object",
	  "properties": {
		"satellite_id": { "type": "string", "format": "uuid" },
		"subscription_manager_id": { "type": "string", "format": "uuid" },
		"insights_inventory_id": { "type": "string", "format": "uuid" },
		"ansible_host": { "type": "string", "maxLength": 255 }
	  },
	  "required": []
	}`)

	middleware.SchemaCache.Store("common:host", `{
	  "$schema": "http://json-schema.org/draft-07/schema#",
	  "type": "object",
	  "properties": {
		"workspace_id": { "type": "string" }
	  },
	  "required": [
		"workspace_id"
	  ]
	}`)
	err = middleware.ValidateReportResourceJSON(msg)
	assert.NoError(t, err)
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
				"insights_inventory_id":   "05707922-7b0a-4fe6-982d-6adbc7695b8f",
				"ansible_host":            "host-1",
			}),
			Common: AsStruct(t, map[string]interface{}{
				"workspace_id": "a64d17d0-aec3-410a-acd0-e0b85b22c076",
			}),
		},
	}

	viper.Set("resources.use_cache", true)
	middleware.SchemaCache.Store("config:host", map[string]interface{}{
		"resource_reporters": []string{"hbi"},
	})

	middleware.SchemaCache.Store("host:hbi", `{
	  "$schema": "http://json-schema.org/draft-07/schema#",
	  "type": "object",
	  "properties": {
		"satellite_id": { "type": "string", "format": "uuid" },
		"subscription_manager_id": { "type": "string", "format": "uuid" },
		"insights_inventory_id": { "type": "string", "format": "uuid" },
		"ansible_host": { "type": "string", "maxLength": 255 }
	  },
	  "required": [
		"subscription_manager_id"
		]
	}`)

	middleware.SchemaCache.Store("common:host", `{
	  "$schema": "http://json-schema.org/draft-07/schema#",
	  "type": "object",
	  "properties": {
		"workspace_id": { "type": "string" }
	  },
	  "required": [
		"workspace_id"
	  ]
	}`)

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
			expect: "Missing 'representations'",
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
			expect: "Missing 'reporter'",
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
			expect: "Missing 'common'",
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
			expect: "invalid reporter_type: bar for resource_type: host",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := middleware.ValidateReportResourceJSON(tc.msg)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), tc.expect)
		})
	}
}

func TestValidateReportResourceJSON_SchemaBasedValidation(t *testing.T) {
	viper.Set("resources.use_cache", true)

	// Setup config for k8s_policy with acm reporter
	middleware.SchemaCache.Store("config:k8s_policy", map[string]interface{}{
		"resource_reporters": []string{"acm"},
	})

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
			expectedError: "Missing 'reporter' field in payload - schema for 'k8s_policy:acm' has required fields",
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
			expectedError: "Missing 'common' field in payload - schema for 'k8s_policy' has required fields",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Setup schemas in cache
			middleware.SchemaCache.Store("k8s_policy:acm", tc.reporterSchema)
			middleware.SchemaCache.Store("common:k8s_policy", tc.commonSchema)

			// Test the function
			err := middleware.ValidateReportResourceJSON(tc.msg)

			if tc.expectError {
				assert.Error(t, err)
				if tc.expectedError != "" {
					assert.Contains(t, err.Error(), tc.expectedError)
				}
			} else {
				assert.NoError(t, err)
			}

			// Cleanup
			middleware.SchemaCache.Delete("k8s_policy:acm")
			middleware.SchemaCache.Delete("common:k8s_policy")
		})
	}
}
