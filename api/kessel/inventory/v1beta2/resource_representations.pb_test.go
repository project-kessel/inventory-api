package v1beta2_test

import (
	"encoding/json"
	"testing"

	"github.com/project-kessel/inventory-api/api/kessel/inventory/v1beta2"
	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/types/known/structpb"
)

// common and reporter representation data is tested for validity in the middleware layer;
// At the API layer these can be invalid but will be caught later on.

func TestResourceRepresentations_MarshalUnmarshalJSON(t *testing.T) {
	commonStruct, _ := structpb.NewStruct(map[string]interface{}{"hostname": "web-01"})
	reporterStruct, _ := structpb.NewStruct(map[string]interface{}{"reporter_type": "hbi"})

	original := &v1beta2.ResourceRepresentations{
		Metadata: &v1beta2.RepresentationMetadata{
			LocalResourceId: "host-123",
			ApiHref:         "/api/hosts/host-123",
		},
		Common:   commonStruct,
		Reporter: reporterStruct,
	}

	data, err := json.Marshal(original)
	assert.NoError(t, err)

	var decoded v1beta2.ResourceRepresentations
	err = json.Unmarshal(data, &decoded)
	assert.NoError(t, err)
	assert.Equal(t, "host-123", decoded.GetMetadata().GetLocalResourceId())
	assert.Equal(t, "web-01", decoded.GetCommon().Fields["hostname"].GetStringValue())
	assert.Equal(t, "hbi", decoded.GetReporter().Fields["reporter_type"].GetStringValue())
}

func TestResourceRepresentations_MissingMetadata(t *testing.T) {
	jsonData := `{
		"common": {"hostname": "no-meta"},
		"reporter": {"reporter_type": "xyz"}
	}`

	var rr v1beta2.ResourceRepresentations
	err := json.Unmarshal([]byte(jsonData), &rr)
	assert.NoError(t, err)

	assert.Nil(t, rr.Metadata)
	assert.Equal(t, "no-meta", rr.GetCommon().Fields["hostname"].GetStringValue())
}

func TestResourceRepresentations_InvalidJSON(t *testing.T) {
	invalidJSON := `{
		"metadata": {
			"local_resource_id": "bad-host",
			"api_href": 123
		},
		"common": {"hostname": "bad-host"}
	}`

	var rr v1beta2.ResourceRepresentations
	err := json.Unmarshal([]byte(invalidJSON), &rr)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot unmarshal number into Go struct field")
}
