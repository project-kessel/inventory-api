package v1beta2_test

import (
	"testing"

	"github.com/project-kessel/inventory-api/api/kessel/inventory/v1beta2"
	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/structpb"
)

func TestReportResourceRequestValidation(t *testing.T) {
	tests := []struct {
		name        string
		req         *v1beta2.ReportResourceRequest
		expectError bool
	}{
		{
			name: "valid request",
			req: &v1beta2.ReportResourceRequest{
				Type:               "host_01",
				ReporterType:       "hbi",
				ReporterInstanceId: "instance_1",
				Representations:    sampleRepresentations(t),
				WriteVisibility:    v1beta2.WriteVisibility_IMMEDIATE,
			},
			expectError: false,
		},
		{
			name: "missing required fields",
			req: &v1beta2.ReportResourceRequest{
				// missing type, reporter_type, etc.
				Representations: sampleRepresentations(t),
			},
			expectError: true,
		},
		{
			name: "invalid type (bad pattern)",
			req: &v1beta2.ReportResourceRequest{
				Type:               "invalid type!", // space and exclamation are disallowed
				ReporterType:       "acm",
				ReporterInstanceId: "instance_2",
				Representations:    sampleRepresentations(t),
				WriteVisibility:    v1beta2.WriteVisibility_IMMEDIATE,
			},
			expectError: true,
		},
		{
			name: "missing representations",
			req: &v1beta2.ReportResourceRequest{
				Type:               "host",
				ReporterType:       "hbi",
				ReporterInstanceId: "inst",
				// representations is nil
				WriteVisibility: v1beta2.WriteVisibility_MINIMIZE_LATENCY,
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.req.Validate()
			if tt.expectError {
				assert.Error(t, err, "expected validation error but got none")
			} else {
				assert.NoError(t, err, "expected no validation error but got one")
			}

			// Test serialization round-trip
			bytes, marshalErr := proto.Marshal(tt.req)
			assert.NoError(t, marshalErr, "marshalling failed")

			var roundTrip v1beta2.ReportResourceRequest
			unmarshalErr := proto.Unmarshal(bytes, &roundTrip)
			assert.NoError(t, unmarshalErr, "unmarshalling failed")
			assert.True(t, proto.Equal(tt.req, &roundTrip), "unmarshalled object differs from original")
		})
	}
}

func sampleRepresentations(t *testing.T) *v1beta2.ResourceRepresentations {
	t.Helper()
	commonStruct, err := structpb.NewStruct(map[string]interface{}{
		"hostname": "example-host",
	})
	assert.NoError(t, err)

	reporterStruct, err := structpb.NewStruct(map[string]interface{}{
		"os": "linux",
	})
	assert.NoError(t, err)

	return &v1beta2.ResourceRepresentations{
		Common:   commonStruct,
		Reporter: reporterStruct,
	}
}
