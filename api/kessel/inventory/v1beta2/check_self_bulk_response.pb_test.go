package v1beta2

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/proto"
)

func TestCheckSelfBulkResponseItem_Nil(t *testing.T) {
	var item *CheckSelfBulkResponseItem
	assert.Equal(t, Allowed_ALLOWED_UNSPECIFIED, item.GetAllowed())
}

func TestCheckSelfBulkResponseItem_ZeroStruct(t *testing.T) {
	var item CheckSelfBulkResponseItem
	assert.Equal(t, Allowed_ALLOWED_UNSPECIFIED, item.GetAllowed())
}

func TestCheckSelfBulkResponseItem_GetAllowed(t *testing.T) {
	item := &CheckSelfBulkResponseItem{Allowed: Allowed_ALLOWED_TRUE}
	assert.Equal(t, Allowed_ALLOWED_TRUE, item.GetAllowed())
}

func TestCheckSelfBulkResponseItem_ProtoMessage(t *testing.T) {
	var item interface{} = &CheckSelfBulkResponseItem{}
	_, ok := item.(proto.Message)
	assert.True(t, ok)
}

func TestCheckSelfBulkResponsePair_Nil(t *testing.T) {
	var pair *CheckSelfBulkResponsePair
	assert.Nil(t, pair.GetRequest())
	assert.Nil(t, pair.GetResponse())
	assert.Nil(t, pair.GetItem())
	assert.Nil(t, pair.GetError())
}

func TestCheckSelfBulkResponsePair_ZeroStruct(t *testing.T) {
	var pair CheckSelfBulkResponsePair
	assert.Nil(t, pair.GetRequest())
	assert.Nil(t, pair.GetResponse())
	assert.Nil(t, pair.GetItem())
	assert.Nil(t, pair.GetError())
}

func TestCheckSelfBulkResponsePair_WithItem(t *testing.T) {
	pair := &CheckSelfBulkResponsePair{
		Request:  &CheckSelfBulkRequestItem{Relation: "view"},
		Response: &CheckSelfBulkResponsePair_Item{Item: &CheckSelfBulkResponseItem{Allowed: Allowed_ALLOWED_TRUE}},
	}
	assert.NotNil(t, pair.GetRequest())
	assert.Equal(t, "view", pair.GetRequest().GetRelation())
	assert.NotNil(t, pair.GetItem())
	assert.Equal(t, Allowed_ALLOWED_TRUE, pair.GetItem().GetAllowed())
	assert.Nil(t, pair.GetError())
}

func TestCheckSelfBulkResponsePair_ProtoMessage(t *testing.T) {
	var pair interface{} = &CheckSelfBulkResponsePair{}
	_, ok := pair.(proto.Message)
	assert.True(t, ok)
}

func TestCheckSelfBulkResponse_Nil(t *testing.T) {
	var resp *CheckSelfBulkResponse
	assert.Nil(t, resp.GetPairs())
	assert.Nil(t, resp.GetConsistencyToken())
}

func TestCheckSelfBulkResponse_ZeroStruct(t *testing.T) {
	var resp CheckSelfBulkResponse
	assert.Nil(t, resp.GetPairs())
	assert.Nil(t, resp.GetConsistencyToken())
}

func TestCheckSelfBulkResponse_Reset(t *testing.T) {
	resp := &CheckSelfBulkResponse{
		Pairs: []*CheckSelfBulkResponsePair{
			{Request: &CheckSelfBulkRequestItem{Relation: "view"}},
		},
	}
	resp.Reset()
	assert.Nil(t, resp.Pairs)
}

func TestCheckSelfBulkResponse_String(t *testing.T) {
	resp := &CheckSelfBulkResponse{
		Pairs: []*CheckSelfBulkResponsePair{
			{Request: &CheckSelfBulkRequestItem{Relation: "view"}},
		},
	}
	s := resp.String()
	assert.NotEmpty(t, s)
	assert.Contains(t, s, "view")
}

func TestCheckSelfBulkResponse_ProtoMessage(t *testing.T) {
	var resp interface{} = &CheckSelfBulkResponse{}
	_, ok := resp.(proto.Message)
	assert.True(t, ok)
}

func TestCheckSelfBulkResponse_ProtoReflect(t *testing.T) {
	resp := &CheckSelfBulkResponse{
		Pairs: []*CheckSelfBulkResponsePair{
			{Request: &CheckSelfBulkRequestItem{Relation: "view"}},
		},
	}
	m := resp.ProtoReflect()
	assert.NotNil(t, m)
	assert.Len(t, m.Interface().(*CheckSelfBulkResponse).Pairs, 1)
}

func TestCheckSelfBulkResponse_MixedAllowedValues(t *testing.T) {
	resp := &CheckSelfBulkResponse{
		Pairs: []*CheckSelfBulkResponsePair{
			{
				Request:  &CheckSelfBulkRequestItem{Object: &ResourceReference{ResourceType: "host", ResourceId: "host-1"}, Relation: "view"},
				Response: &CheckSelfBulkResponsePair_Item{Item: &CheckSelfBulkResponseItem{Allowed: Allowed_ALLOWED_TRUE}},
			},
			{
				Request:  &CheckSelfBulkRequestItem{Object: &ResourceReference{ResourceType: "host", ResourceId: "host-2"}, Relation: "edit"},
				Response: &CheckSelfBulkResponsePair_Item{Item: &CheckSelfBulkResponseItem{Allowed: Allowed_ALLOWED_FALSE}},
			},
			{
				Request:  &CheckSelfBulkRequestItem{Object: &ResourceReference{ResourceType: "vm", ResourceId: "vm-1"}, Relation: "delete"},
				Response: &CheckSelfBulkResponsePair_Item{Item: &CheckSelfBulkResponseItem{Allowed: Allowed_ALLOWED_TRUE}},
			},
		},
	}

	assert.Len(t, resp.GetPairs(), 3)

	// First pair: allowed
	assert.Equal(t, "host-1", resp.GetPairs()[0].GetRequest().GetObject().GetResourceId())
	assert.Equal(t, "view", resp.GetPairs()[0].GetRequest().GetRelation())
	assert.Equal(t, Allowed_ALLOWED_TRUE, resp.GetPairs()[0].GetItem().GetAllowed())

	// Second pair: denied
	assert.Equal(t, "host-2", resp.GetPairs()[1].GetRequest().GetObject().GetResourceId())
	assert.Equal(t, "edit", resp.GetPairs()[1].GetRequest().GetRelation())
	assert.Equal(t, Allowed_ALLOWED_FALSE, resp.GetPairs()[1].GetItem().GetAllowed())

	// Third pair: allowed
	assert.Equal(t, "vm-1", resp.GetPairs()[2].GetRequest().GetObject().GetResourceId())
	assert.Equal(t, "delete", resp.GetPairs()[2].GetRequest().GetRelation())
	assert.Equal(t, Allowed_ALLOWED_TRUE, resp.GetPairs()[2].GetItem().GetAllowed())
}
