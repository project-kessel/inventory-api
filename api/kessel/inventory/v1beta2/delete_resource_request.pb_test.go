package v1beta2_test

import (
	"encoding/json"
	"testing"

	"github.com/project-kessel/inventory-api/api/kessel/inventory/v1beta2"
	"github.com/stretchr/testify/assert"
)

// Test full DeleteResourceRequest with ReporterReference present
func TestDeleteResourceRequest_Full(t *testing.T) {
	req := &v1beta2.DeleteResourceRequest{
		Reference: &v1beta2.ResourceReference{
			ResourceType: "host",
			ResourceId:   "host-123",
			Reporter: &v1beta2.ReporterReference{
				Type: "hbi",
			},
		},
	}

	data, err := json.Marshal(req)
	assert.NoError(t, err)

	var out v1beta2.DeleteResourceRequest
	err = json.Unmarshal(data, &out)
	assert.NoError(t, err)
	assert.Equal(t, "host", out.GetReference().GetResourceType())
	assert.Equal(t, "host-123", out.GetReference().GetResourceId())
	assert.Equal(t, "hbi", out.GetReference().GetReporter().GetType())
}

// Test minimal DeleteResourceRequest (no reporter)
func TestDeleteResourceRequest_Minimal(t *testing.T) {
	req := &v1beta2.DeleteResourceRequest{
		Reference: &v1beta2.ResourceReference{
			ResourceType: "k8s_cluster",
			ResourceId:   "cluster-abc",
		},
	}

	data, err := json.Marshal(req)
	assert.NoError(t, err)

	var out v1beta2.DeleteResourceRequest
	err = json.Unmarshal(data, &out)
	assert.NoError(t, err)
	assert.Equal(t, "k8s_cluster", out.GetReference().GetResourceType())
	assert.Equal(t, "cluster-abc", out.GetReference().GetResourceId())
	assert.Nil(t, out.GetReference().GetReporter())
}

// Test missing reference field
func TestDeleteResourceRequest_MissingReference(t *testing.T) {
	jsonData := `{}`

	var out v1beta2.DeleteResourceRequest
	err := json.Unmarshal([]byte(jsonData), &out)
	assert.NoError(t, err)
	assert.Nil(t, out.Reference)
}

// Negative test: reporter field is the wrong type
func TestDeleteResourceRequest_InvalidReporterType(t *testing.T) {
	jsonData := `{
		"reference": {
			"resource_type": "host",
			"resource_id": "123",
			"reporter": "should-be-object"
		}
	}`

	var out v1beta2.DeleteResourceRequest
	err := json.Unmarshal([]byte(jsonData), &out)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot unmarshal string into Go struct field")
}

// Negative test: resource_id is an integer instead of a string
func TestDeleteResourceRequest_InvalidResourceIDType(t *testing.T) {
	jsonData := `{
		"reference": {
			"resource_type": "host",
			"resource_id": 42
		}
	}`

	var out v1beta2.DeleteResourceRequest
	err := json.Unmarshal([]byte(jsonData), &out)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot unmarshal number into Go struct field")
}

func TestDeleteResourceRequest_NilReferenceExplicit(t *testing.T) {
	req := &v1beta2.DeleteResourceRequest{
		Reference: nil,
	}
	data, err := json.Marshal(req)
	assert.NoError(t, err)

	var out v1beta2.DeleteResourceRequest
	err = json.Unmarshal(data, &out)
	assert.NoError(t, err)
	assert.Nil(t, out.Reference)
}

func TestDeleteResourceRequest_ResetAndValidate(t *testing.T) {
	req := &v1beta2.DeleteResourceRequest{
		Reference: &v1beta2.ResourceReference{
			ResourceType: "host",
			ResourceId:   "host-123",
		},
	}

	assert.NotNil(t, req.Reference)
	assert.Equal(t, "host", req.Reference.ResourceType)
	assert.Equal(t, "host-123", req.Reference.ResourceId)

	req.Reset()
	assert.Nil(t, req.Reference)
}
