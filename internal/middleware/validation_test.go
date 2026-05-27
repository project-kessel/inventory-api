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
