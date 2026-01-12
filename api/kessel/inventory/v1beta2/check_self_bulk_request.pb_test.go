package v1beta2

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/proto"
)

func TestCheckSelfBulkRequestItem_Nil(t *testing.T) {
	var item *CheckSelfBulkRequestItem
	assert.Nil(t, item.GetObject())
	assert.Equal(t, "", item.GetRelation())
}

func TestCheckSelfBulkRequestItem_ZeroStruct(t *testing.T) {
	var item CheckSelfBulkRequestItem
	assert.Nil(t, item.GetObject())
	assert.Equal(t, "", item.GetRelation())
}

func TestCheckSelfBulkRequestItem_Reset(t *testing.T) {
	item := &CheckSelfBulkRequestItem{
		Object:   &ResourceReference{ResourceType: "host", ResourceId: "123"},
		Relation: "view",
	}
	item.Reset()
	assert.Nil(t, item.Object)
	assert.Equal(t, "", item.Relation)
}

func TestCheckSelfBulkRequestItem_ProtoMessage(t *testing.T) {
	var item interface{} = &CheckSelfBulkRequestItem{}
	_, ok := item.(proto.Message)
	assert.True(t, ok)
}

func TestCheckSelfBulkRequest_Nil(t *testing.T) {
	var req *CheckSelfBulkRequest
	assert.Nil(t, req.GetItems())
	assert.Nil(t, req.GetConsistencyToken())
}

func TestCheckSelfBulkRequest_ZeroStruct(t *testing.T) {
	var req CheckSelfBulkRequest
	assert.Nil(t, req.GetItems())
	assert.Nil(t, req.GetConsistencyToken())
}

func TestCheckSelfBulkRequest_Reset(t *testing.T) {
	req := &CheckSelfBulkRequest{
		Items: []*CheckSelfBulkRequestItem{
			{Object: &ResourceReference{ResourceType: "host"}, Relation: "view"},
		},
	}
	req.Reset()
	assert.Nil(t, req.Items)
}

func TestCheckSelfBulkRequest_String(t *testing.T) {
	req := &CheckSelfBulkRequest{
		Items: []*CheckSelfBulkRequestItem{
			{Object: &ResourceReference{ResourceType: "host"}, Relation: "view"},
		},
	}
	s := req.String()
	assert.NotEmpty(t, s)
	assert.Contains(t, s, "host")
}

func TestCheckSelfBulkRequest_ProtoMessage(t *testing.T) {
	var req interface{} = &CheckSelfBulkRequest{}
	_, ok := req.(proto.Message)
	assert.True(t, ok)
}

func TestCheckSelfBulkRequest_ProtoReflect(t *testing.T) {
	req := &CheckSelfBulkRequest{
		Items: []*CheckSelfBulkRequestItem{
			{Relation: "view"},
		},
	}
	m := req.ProtoReflect()
	assert.NotNil(t, m)
	assert.Len(t, m.Interface().(*CheckSelfBulkRequest).Items, 1)
}

func TestCheckSelfBulkRequest_MultipleItems(t *testing.T) {
	req := &CheckSelfBulkRequest{
		Items: []*CheckSelfBulkRequestItem{
			{Object: &ResourceReference{ResourceType: "host", ResourceId: "host-1"}, Relation: "view"},
			{Object: &ResourceReference{ResourceType: "host", ResourceId: "host-2"}, Relation: "edit"},
			{Object: &ResourceReference{ResourceType: "vm", ResourceId: "vm-1"}, Relation: "delete"},
		},
	}
	assert.Len(t, req.GetItems(), 3)
	assert.Equal(t, "host-1", req.GetItems()[0].GetObject().GetResourceId())
	assert.Equal(t, "view", req.GetItems()[0].GetRelation())
	assert.Equal(t, "host-2", req.GetItems()[1].GetObject().GetResourceId())
	assert.Equal(t, "edit", req.GetItems()[1].GetRelation())
	assert.Equal(t, "vm-1", req.GetItems()[2].GetObject().GetResourceId())
	assert.Equal(t, "delete", req.GetItems()[2].GetRelation())
}
