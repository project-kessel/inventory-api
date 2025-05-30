package v1beta2_test

import (
	"encoding/json"
	"testing"

	"github.com/project-kessel/inventory-api/api/kessel/inventory/v1beta2"
	"github.com/stretchr/testify/assert"
)

func minimalResourceRepresentations() *v1beta2.ResourceRepresentations {
	return &v1beta2.ResourceRepresentations{}
}

func TestReportResourceRequest_MarshalUnmarshalFull(t *testing.T) {
	req := &v1beta2.ReportResourceRequest{
		InventoryId:        strPtr("inv-123"),
		Type:               "k8s_cluster",
		ReporterType:       "acm",
		ReporterInstanceId: "acm-1",
		Representations:    minimalResourceRepresentations(),
		WriteVisibility:    v1beta2.WriteVisibility_IMMEDIATE,
	}
	data, err := json.Marshal(req)
	assert.NoError(t, err)

	var out v1beta2.ReportResourceRequest
	err = json.Unmarshal(data, &out)
	assert.NoError(t, err)

	assert.Equal(t, "inv-123", out.GetInventoryId())
	assert.Equal(t, "k8s_cluster", out.GetType())
	assert.Equal(t, "acm", out.GetReporterType())
	assert.Equal(t, "acm-1", out.GetReporterInstanceId())
	assert.Equal(t, v1beta2.WriteVisibility_IMMEDIATE, out.GetWriteVisibility())
	assert.NotNil(t, out.GetRepresentations())
}

func TestReportResourceRequest_Minimal(t *testing.T) {
	req := &v1beta2.ReportResourceRequest{
		Type:               "host",
		ReporterType:       "hbi",
		ReporterInstanceId: "hbi-1",
		Representations:    minimalResourceRepresentations(),
	}
	data, err := json.Marshal(req)
	assert.NoError(t, err)

	var out v1beta2.ReportResourceRequest
	err = json.Unmarshal(data, &out)
	assert.NoError(t, err)

	assert.Equal(t, "", out.GetInventoryId())
	assert.Equal(t, v1beta2.WriteVisibility_WRITE_VISIBILITY_UNSPECIFIED, out.GetWriteVisibility())
	assert.Equal(t, "host", out.GetType())
	assert.Equal(t, "hbi", out.GetReporterType())
}

func TestReportResourceRequest_MissingRepresentations(t *testing.T) {
	req := &v1beta2.ReportResourceRequest{
		Type:               "host",
		ReporterType:       "hbi",
		ReporterInstanceId: "hbi-1",
	}
	data, err := json.Marshal(req)
	assert.NoError(t, err)

	var out v1beta2.ReportResourceRequest
	err = json.Unmarshal(data, &out)
	assert.NoError(t, err)
	assert.Nil(t, out.Representations)
}

func TestReportResourceRequest_InvalidWriteVisibility(t *testing.T) {
	// Unknown enum should parse as zero value (unspecified)
	jsonData := `{
		"type": "host",
		"reporter_type": "hbi",
		"reporter_instance_id": "abc",
		"write_visibility": 999
	}`
	var out v1beta2.ReportResourceRequest
	err := json.Unmarshal([]byte(jsonData), &out)
	assert.NoError(t, err)
	assert.Equal(t, v1beta2.WriteVisibility(999), out.GetWriteVisibility())
}

func TestReportResourceRequest_UnknownFieldsIgnored(t *testing.T) {
	jsonData := `{
		"type": "host",
		"reporter_type": "hbi",
		"reporter_instance_id": "abc",
		"unknown_field": "ignored"
	}`
	var out v1beta2.ReportResourceRequest
	err := json.Unmarshal([]byte(jsonData), &out)
	assert.NoError(t, err)
	assert.Equal(t, "host", out.GetType())
	assert.Equal(t, "hbi", out.GetReporterType())
}

func TestReportResourceRequest_InvalidType(t *testing.T) {
	jsonData := `{
		"type": 42,
		"reporter_type": "hbi",
		"reporter_instance_id": "abc"
	}`
	var out v1beta2.ReportResourceRequest
	err := json.Unmarshal([]byte(jsonData), &out)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot unmarshal number")
}

func TestReportResourceRequest_Reset(t *testing.T) {
	req := &v1beta2.ReportResourceRequest{
		Type:               "host",
		ReporterType:       "hbi",
		ReporterInstanceId: "abc",
	}
	req.Reset()
	assert.Equal(t, "", req.Type)
	assert.Equal(t, "", req.ReporterType)
	assert.Equal(t, "", req.ReporterInstanceId)
}

func TestReportResourceRequest_String(t *testing.T) {
	req := &v1beta2.ReportResourceRequest{
		Type: "host",
	}
	str := req.String()
	assert.Contains(t, str, "host")
}

// Utility
func strPtr(s string) *string { return &s }
