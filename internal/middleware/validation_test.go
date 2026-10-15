package middleware

import (
	"context"
	"testing"

	"buf.build/go/protovalidate"
	"github.com/go-kratos/kratos/v2/errors"
	pb "github.com/project-kessel/inventory-api/api/kessel/inventory/v1beta2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/structpb"
)

func TestValidation_ValidRequest(t *testing.T) {
	t.Parallel()

	validator, err := protovalidate.New()
	require.NoError(t, err)

	mw := Validation(validator)
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return "ok", nil
	}

	req := &pb.ReportResourceRequest{
		Type:               "host",
		ReporterInstanceId: "test-instance",
		ReporterType:       "hbi",
		Representations: &pb.ResourceRepresentations{
			Metadata: &pb.RepresentationMetadata{
				LocalResourceId: "test-123",
				ApiHref:         "/api/test",
			},
		},
	}

	resp, err := mw(handler)(context.Background(), req)
	assert.NoError(t, err)
	assert.Equal(t, "ok", resp)
}

func TestValidation_InvalidRequest(t *testing.T) {
	t.Parallel()

	validator, err := protovalidate.New()
	require.NoError(t, err)

	mw := Validation(validator)
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return "ok", nil
	}

	req := &pb.ReportResourceRequest{}

	_, err = mw(handler)(context.Background(), req)
	assert.Error(t, err)
	assert.True(t, errors.IsBadRequest(err))
}

func TestValidation_AddResourceRelationshipsRequest(t *testing.T) {
	t.Parallel()

	validator, err := protovalidate.New()
	require.NoError(t, err)

	mw := Validation(validator)
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return "ok", nil
	}

	validReporter, err := structpb.NewStruct(map[string]interface{}{
		"direct_service_preferences": []interface{}{"550e8400-e29b-41d4-a716-446655440000"},
	})
	require.NoError(t, err)

	validCommon, err := structpb.NewStruct(map[string]interface{}{
		"workspace_id": "7c9e6679-7425-40de-944b-e07fc1f90ae7",
	})
	require.NoError(t, err)

	emptyStruct, err := structpb.NewStruct(map[string]interface{}{})
	require.NoError(t, err)

	tests := []struct {
		name      string
		reporter  *structpb.Struct
		common    *structpb.Struct
		expectErr bool
	}{
		{
			name:      "reporter only is valid",
			reporter:  validReporter,
			expectErr: false,
		},
		{
			name:      "common only is valid",
			common:    validCommon,
			expectErr: false,
		},
		{
			name:      "both provided is valid",
			reporter:  validReporter,
			common:    validCommon,
			expectErr: false,
		},
		{
			name:      "both absent is rejected",
			expectErr: true,
		},
		{
			name:      "empty reporter struct with no common is rejected",
			reporter:  emptyStruct,
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			req := &pb.AddResourceRelationshipsRequest{
				Type:               "workspace",
				ReporterType:       "features",
				ReporterInstanceId: "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
				LocalResourceId:    "root",
				Reporter:           tt.reporter,
				Common:             tt.common,
			}

			_, err := mw(handler)(context.Background(), req)
			if tt.expectErr {
				assert.Error(t, err)
				assert.True(t, errors.IsBadRequest(err))
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidation_RemoveResourceRelationshipsRequest(t *testing.T) {
	t.Parallel()

	validator, err := protovalidate.New()
	require.NoError(t, err)

	mw := Validation(validator)
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return "ok", nil
	}

	validReporter, err := structpb.NewStruct(map[string]interface{}{
		"direct_service_preferences": []interface{}{"6ba7b810-9dad-11d1-80b4-00c04fd430c8"},
	})
	require.NoError(t, err)

	tests := []struct {
		name      string
		reporter  *structpb.Struct
		common    *structpb.Struct
		expectErr bool
	}{
		{
			name:      "reporter only is valid",
			reporter:  validReporter,
			expectErr: false,
		},
		{
			name:      "both absent is rejected",
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			req := &pb.RemoveResourceRelationshipsRequest{
				Type:               "workspace",
				ReporterType:       "features",
				ReporterInstanceId: "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
				LocalResourceId:    "root",
				Reporter:           tt.reporter,
				Common:             tt.common,
			}

			_, err := mw(handler)(context.Background(), req)
			if tt.expectErr {
				assert.Error(t, err)
				assert.True(t, errors.IsBadRequest(err))
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestSanitizeReportResourceRequest(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name             string
		reporterInput    map[string]interface{}
		expectedReporter map[string]interface{}
	}{
		{
			name:             "reporter nulls removed",
			reporterInput:    map[string]interface{}{"satellite_id": "sat-123", "stale_key": nil},
			expectedReporter: map[string]interface{}{"satellite_id": "sat-123"},
		},
		{
			name: "nil representations",
		},
		{
			name:             "nil reporter",
			reporterInput:    nil,
			expectedReporter: nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			req := &pb.ReportResourceRequest{
				Type:               "host",
				ReporterInstanceId: "test-instance",
				ReporterType:       "hbi",
			}

			if tc.reporterInput != nil {
				s, err := structpb.NewStruct(tc.reporterInput)
				require.NoError(t, err)
				req.Representations = &pb.ResourceRepresentations{
					Metadata: &pb.RepresentationMetadata{
						LocalResourceId: "test-123",
						ApiHref:         "/api/test",
					},
					Reporter: s,
				}
			}

			err := sanitizeReportResourceRequest(req)
			assert.NoError(t, err)

			if tc.expectedReporter != nil {
				assert.Equal(t, tc.expectedReporter, req.GetRepresentations().GetReporter().AsMap())
			}
		})
	}
}
