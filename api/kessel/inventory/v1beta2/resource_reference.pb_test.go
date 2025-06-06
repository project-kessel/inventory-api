package v1beta2_test

import (
	"encoding/json"
	"google.golang.org/protobuf/proto"
	"testing"

	"github.com/project-kessel/inventory-api/api/kessel/inventory/v1beta2"
	"github.com/stretchr/testify/assert"
)

// Test successful marshal/unmarshal round-trip with all fields.
func TestResourceReference_Full(t *testing.T) {
	ref := &v1beta2.ResourceReference{
		ResourceType: "host",
		ResourceId:   "123",
		Reporter: &v1beta2.ReporterReference{
			Type: "hbi",
		},
	}

	data, err := json.Marshal(ref)
	assert.NoError(t, err)

	var out v1beta2.ResourceReference
	err = json.Unmarshal(data, &out)
	assert.NoError(t, err)
	assert.Equal(t, "host", out.ResourceType)
	assert.Equal(t, "123", out.ResourceId)
	assert.NotNil(t, out.Reporter)
	assert.Equal(t, "hbi", out.Reporter.Type)
}

// Test with only required fields (no reporter).
func TestResourceReference_Minimal(t *testing.T) {
	ref := &v1beta2.ResourceReference{
		ResourceType: "k8s_cluster",
		ResourceId:   "cluster-001",
	}

	data, err := json.Marshal(ref)
	assert.NoError(t, err)

	var out v1beta2.ResourceReference
	err = json.Unmarshal(data, &out)
	assert.NoError(t, err)
	assert.Equal(t, "k8s_cluster", out.ResourceType)
	assert.Equal(t, "cluster-001", out.ResourceId)
	assert.Nil(t, out.Reporter)
}

// Test invalid JSON: resource_id is missing
func TestResourceReference_MissingResourceID(t *testing.T) {
	jsonData := `{"resource_type": "host"}`
	var ref v1beta2.ResourceReference
	err := json.Unmarshal([]byte(jsonData), &ref)
	assert.NoError(t, err) // Unmarshal will succeed, but validation will fail elsewhere
	assert.Equal(t, "host", ref.ResourceType)
	assert.Empty(t, ref.ResourceId)
}

// Test invalid JSON type: reporter should be an object, not a string
func TestResourceReference_InvalidReporterType(t *testing.T) {
	jsonData := `{
		"resource_type": "host",
		"resource_id": "abc",
		"reporter": "should-be-an-object"
	}`
	var ref v1beta2.ResourceReference
	err := json.Unmarshal([]byte(jsonData), &ref)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot unmarshal string into Go struct field")
}

// Test empty object
func TestResourceReference_Empty(t *testing.T) {
	var ref v1beta2.ResourceReference
	assert.Empty(t, ref.ResourceType)
	assert.Empty(t, ref.ResourceId)
	assert.Nil(t, ref.Reporter)
}

func TestResourceReference_Reset(t *testing.T) {
	ref := &v1beta2.ResourceReference{
		ResourceType: "host",
		ResourceId:   "host-99",
		Reporter:     &v1beta2.ReporterReference{Type: "foo"},
	}

	assert.Equal(t, "host", ref.ResourceType)
	assert.Equal(t, "host-99", ref.ResourceId)
	assert.NotNil(t, ref.Reporter)

	refString1 := ref.String()
	assert.Contains(t, refString1, "host-99")

	ref.Reset()
	assert.Empty(t, ref.ResourceType)
	assert.Empty(t, ref.ResourceId)
	assert.Nil(t, ref.Reporter)

	refString2 := ref.String()
	assert.Contains(t, refString2, "")
}

func TestResourceReference_ProtoMessage(t *testing.T) {
	var ref interface{} = &v1beta2.ResourceReference{}
	_, ok := ref.(proto.Message)
	assert.True(t, ok)
}
