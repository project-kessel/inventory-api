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

func TestSanitizeReportResourceRequest_RemovesNulls(t *testing.T) {
	t.Parallel()

	reporterData, err := structpb.NewStruct(map[string]interface{}{
		"satellite_id": "sat-123",
		"stale_key":    nil,
	})
	require.NoError(t, err)

	req := &pb.ReportResourceRequest{
		Type:               "host",
		ReporterInstanceId: "test-instance",
		ReporterType:       "hbi",
		Representations: &pb.ResourceRepresentations{
			Metadata: &pb.RepresentationMetadata{
				LocalResourceId: "test-123",
				ApiHref:         "/api/test",
			},
			Reporter: reporterData,
		},
	}

	err = sanitizeReportResourceRequest(req)
	assert.NoError(t, err)

	sanitizedMap := req.GetRepresentations().GetReporter().AsMap()
	assert.Equal(t, "sat-123", sanitizedMap["satellite_id"])
	assert.NotContains(t, sanitizedMap, "stale_key")
}

func TestSanitizeReportResourceRequest_NilRepresentations(t *testing.T) {
	t.Parallel()

	req := &pb.ReportResourceRequest{
		Type:               "host",
		ReporterInstanceId: "test-instance",
		ReporterType:       "hbi",
	}

	err := sanitizeReportResourceRequest(req)
	assert.NoError(t, err)
}

func TestSanitizeReportResourceRequest_NilReporter(t *testing.T) {
	t.Parallel()

	commonData, err := structpb.NewStruct(map[string]interface{}{
		"workspace_id": "ws-1",
	})
	require.NoError(t, err)

	req := &pb.ReportResourceRequest{
		Type:               "host",
		ReporterInstanceId: "test-instance",
		ReporterType:       "hbi",
		Representations: &pb.ResourceRepresentations{
			Metadata: &pb.RepresentationMetadata{
				LocalResourceId: "test-123",
			},
			Common: commonData,
		},
	}

	err = sanitizeReportResourceRequest(req)
	assert.NoError(t, err)
}
