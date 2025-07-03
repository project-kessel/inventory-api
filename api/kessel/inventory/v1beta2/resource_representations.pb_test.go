package v1beta2_test

import (
	"encoding/json"
	"testing"

	"google.golang.org/protobuf/proto"

	"github.com/project-kessel/inventory-api/api/kessel/inventory/v1beta2"
	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/types/known/structpb"
)

// common and reporter representation data is tested for validity in the middleware layer;
// At the API layer these can be invalid but will be caught later on.

func TestResourceRepresentations_MarshalUnmarshalJSON(t *testing.T) {
	commonStruct, _ := structpb.NewStruct(map[string]interface{}{"workspace_id": "1"})
	reporterStruct, _ := structpb.NewStruct(map[string]interface{}{"satellite_id": "2"})

	original := &v1beta2.ResourceRepresentations{
		Metadata: &v1beta2.RepresentationMetadata{
			LocalResourceId: "host-123",
			ApiHref:         "www.host-123.com",
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
	assert.Equal(t, "www.host-123.com", decoded.GetMetadata().GetApiHref())
	assert.Equal(t, "1", decoded.GetCommon().Fields["workspace_id"].GetStringValue())
	assert.Equal(t, "2", decoded.GetReporter().Fields["satellite_id"].GetStringValue())
}

func TestResourceRepresentations_MissingMetadata(t *testing.T) {
	jsonData := `{
		"common": {"workspace_id": "3"},
		"reporter": {"satellite_id": "4"}
	}`

	var rr v1beta2.ResourceRepresentations
	err := json.Unmarshal([]byte(jsonData), &rr)
	assert.NoError(t, err)

	assert.Nil(t, rr.Metadata)
	assert.Equal(t, "3", rr.GetCommon().Fields["workspace_id"].GetStringValue())
}

func TestResourceRepresentations_InvalidJSON(t *testing.T) {
	invalidJSON := `{
		"metadata": {
			"local_resource_id": 1,
			"api_href": "www.bad-local-resource-id.com"
		},
		"common": {"workspace_id": "2"}
	}`

	var rr v1beta2.ResourceRepresentations
	err := json.Unmarshal([]byte(invalidJSON), &rr)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot unmarshal number into Go struct field")
}

func TestResourceRepresentations_Reset(t *testing.T) {
	commonStruct, _ := structpb.NewStruct(map[string]interface{}{"workspace_id": "1"})
	reporterStruct, _ := structpb.NewStruct(map[string]interface{}{"satellite_id": "2"})
	rr := &v1beta2.ResourceRepresentations{
		Metadata: &v1beta2.RepresentationMetadata{
			LocalResourceId: "some-id",
			ApiHref:         "some-url",
		},
		Common:   commonStruct,
		Reporter: reporterStruct,
	}

	assert.NotNil(t, rr.Metadata)
	assert.NotNil(t, rr.Common)
	assert.NotNil(t, rr.Reporter)

	rrString1 := rr.String()
	assert.Contains(t, rrString1, "some-id")

	rr.Reset()
	assert.Nil(t, rr.Metadata)
	assert.Nil(t, rr.Common)
	assert.Nil(t, rr.Reporter)

	rrString2 := rr.String()

	assert.Empty(t, rrString2)
	assert.NotContains(t, rrString2, "some-id")

}

func TestResourceRepresentations_ProtoMessage(t *testing.T) {
	var rr interface{} = &v1beta2.ResourceRepresentations{}
	_, ok := rr.(proto.Message)
	assert.True(t, ok)
}

func TestResourceRepresentations_ProtoReflect(t *testing.T) {
	rr := &v1beta2.ResourceRepresentations{}
	m := rr.ProtoReflect()
	assert.NotNil(t, m)
	assert.Equal(t, "ResourceRepresentations", string(m.Descriptor().Name()))
}
