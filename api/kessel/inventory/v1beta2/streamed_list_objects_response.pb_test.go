package v1beta2

import (
	"encoding/json"
	"google.golang.org/protobuf/proto"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStreamedListObjectsResponse_Getters(t *testing.T) {
	obj := &ResourceReference{ResourceType: "host", ResourceId: "123"}
	pagination := &ResponsePagination{ContinuationToken: "abc"}
	consistencyToken := &ConsistencyToken{Token: "zzz"}
	resp := &StreamedListObjectsResponse{
		Object:           obj,
		Pagination:       pagination,
		ConsistencyToken: consistencyToken,
	}
	assert.Equal(t, obj, resp.GetObject())
	assert.Equal(t, pagination, resp.GetPagination())
	assert.Equal(t, consistencyToken, resp.GetConsistencyToken())

	// nil receiver
	var respNil *StreamedListObjectsResponse
	assert.Nil(t, respNil.GetObject())
	assert.Nil(t, respNil.GetPagination())
	assert.Nil(t, respNil.GetConsistencyToken())
}

func TestStreamedListObjectsResponse_Reset(t *testing.T) {
	obj := &ResourceReference{ResourceType: "host"}
	pagination := &ResponsePagination{ContinuationToken: "abc"}
	consistencyToken := &ConsistencyToken{Token: "a"}
	resp := &StreamedListObjectsResponse{
		Object:           obj,
		Pagination:       pagination,
		ConsistencyToken: consistencyToken,
	}
	resp.Reset()
	assert.Nil(t, resp.GetObject())
	assert.Nil(t, resp.GetPagination())
	assert.Nil(t, resp.GetConsistencyToken())
}

func TestStreamedListObjectsResponse_String(t *testing.T) {
	obj := &ResourceReference{ResourceType: "host"}
	resp := &StreamedListObjectsResponse{Object: obj}
	s := resp.String()
	assert.NotEmpty(t, s)
	assert.Contains(t, s, "host")
}

func TestStreamedListObjectsResponse_ProtoMessage(t *testing.T) {
	var resp interface{} = &StreamedListObjectsResponse{}
	_, ok := resp.(proto.Message)
	assert.True(t, ok)
}

func TestStreamedListObjectsResponse_ProtoReflect(t *testing.T) {
	obj := &ResourceReference{ResourceType: "host"}
	resp := &StreamedListObjectsResponse{Object: obj}
	m := resp.ProtoReflect()
	assert.NotNil(t, m)
	assert.Equal(t, "host", m.Interface().(*StreamedListObjectsResponse).GetObject().GetResourceType())
}

func TestStreamedListObjectsResponse_JSONRoundTrip_Full(t *testing.T) {
	obj := &ResourceReference{ResourceType: "host", ResourceId: "123"}
	pagination := &ResponsePagination{ContinuationToken: "def"}
	consistencyToken := &ConsistencyToken{Token: "token"}
	resp := &StreamedListObjectsResponse{
		Object:           obj,
		Pagination:       pagination,
		ConsistencyToken: consistencyToken,
	}
	b, err := json.Marshal(resp)
	assert.NoError(t, err)
	var out StreamedListObjectsResponse
	assert.NoError(t, json.Unmarshal(b, &out))
	assert.NotNil(t, out.GetObject())
	assert.Equal(t, "host", out.GetObject().GetResourceType())
	assert.Equal(t, "123", out.GetObject().GetResourceId())
	assert.NotNil(t, out.GetPagination())
	assert.Equal(t, "def", out.GetPagination().ContinuationToken)
	assert.NotNil(t, out.GetConsistencyToken())
	assert.Equal(t, "token", out.GetConsistencyToken().GetToken())
}

func TestStreamedListObjectsResponse_JSONRoundTrip_NilFields(t *testing.T) {
	resp := &StreamedListObjectsResponse{}
	b, err := json.Marshal(resp)
	assert.NoError(t, err)
	var out StreamedListObjectsResponse
	assert.NoError(t, json.Unmarshal(b, &out))
	assert.Nil(t, out.GetObject())
	assert.Nil(t, out.GetPagination())
	assert.Nil(t, out.GetConsistencyToken())
}

func TestStreamedListObjectsResponse_AllNil(t *testing.T) {
	var resp *StreamedListObjectsResponse
	assert.Nil(t, resp.GetObject())
	assert.Nil(t, resp.GetPagination())
	assert.Nil(t, resp.GetConsistencyToken())
}
