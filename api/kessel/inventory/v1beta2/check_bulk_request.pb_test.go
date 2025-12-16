package v1beta2

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/proto"
)

func TestCheckBulkRequest_FullRoundTrip(t *testing.T) {
	relation := "members"
	cr := &CheckBulkRequest{
		Items: []*CheckBulkRequestItem{
			{
				Object:   &ResourceReference{ResourceType: "host", ResourceId: "host-123"},
				Relation: "view",
				Subject:  &SubjectReference{Relation: &relation, Resource: &ResourceReference{ResourceType: "principal", ResourceId: "sarah"}},
			},
		},
	}

	// Marshal to JSON and back
	data, err := json.Marshal(cr)
	assert.NoError(t, err)

	var out CheckBulkRequest
	err = json.Unmarshal(data, &out)
	assert.NoError(t, err)

	// Check that we have one item
	assert.Len(t, out.GetItems(), 1)

	item := out.GetItems()[0]

	// Check object
	assert.Equal(t, "host", item.GetObject().GetResourceType())
	assert.Equal(t, "host-123", item.GetObject().GetResourceId())

	// Check relation
	assert.Equal(t, "view", item.GetRelation())

	// Check subject
	assert.NotNil(t, item.GetSubject())
	assert.Equal(t, "members", item.GetSubject().GetRelation())
	assert.NotNil(t, item.GetSubject().GetResource())
	assert.Equal(t, "principal", item.GetSubject().GetResource().GetResourceType())
	assert.Equal(t, "sarah", item.GetSubject().GetResource().GetResourceId())
}

func TestCheckBulkRequest_Reset(t *testing.T) {
	cr := &CheckBulkRequest{
		Items: []*CheckBulkRequestItem{
			{
				Object:   &ResourceReference{ResourceType: "vm", ResourceId: "123"},
				Relation: "read",
				Subject:  &SubjectReference{},
			},
		},
	}
	cr.Reset()
	assert.Nil(t, cr.GetItems())
	assert.Nil(t, cr.GetConsistency())
}
func TestCheckBulkRequest_String(t *testing.T) {
	cr := &CheckBulkRequest{
		Items: []*CheckBulkRequestItem{
			{
				Object:   &ResourceReference{ResourceType: "host"},
				Relation: "view",
			},
		},
	}
	s := cr.String()
	assert.NotEmpty(t, s)
	// Optionally, check that it mentions your field values
	assert.Contains(t, s, "host")
	assert.Contains(t, s, "view")
}

func TestCheckBulkRequest_ProtoMessage(t *testing.T) {
	var cr interface{} = &CheckBulkRequest{}
	_, ok := cr.(proto.Message)
	assert.True(t, ok)
}

func TestCheckBulkRequest_ProtoReflect(t *testing.T) {
	cr := &CheckBulkRequest{
		Items: []*CheckBulkRequestItem{
			{
				Object:   &ResourceReference{ResourceType: "host"},
				Relation: "view",
			},
		},
	}
	m := cr.ProtoReflect()
	assert.NotNil(t, m)
	assert.Equal(t, "view", m.Interface().(*CheckBulkRequest).Items[0].Relation)
}

func TestCheckBulkRequest_NilFields(t *testing.T) {
	var cr *CheckBulkRequest
	// All getters should be safe to call on nil and return zero values
	assert.Nil(t, cr.GetItems())
	assert.Nil(t, cr.GetConsistency())
}

func TestCheckBulkRequest_EmptyStruct(t *testing.T) {
	var cr CheckBulkRequest
	// All getters should return zero values, not panic
	assert.Nil(t, cr.GetItems())
	assert.Nil(t, cr.GetConsistency())
}

func TestCheckBulkRequest_SubjectNilResource(t *testing.T) {
	cr := &CheckBulkRequest{
		Items: []*CheckBulkRequestItem{
			{
				Subject: &SubjectReference{Resource: nil},
			},
		},
	}
	assert.Nil(t, cr.GetItems()[0].GetSubject().GetResource())
	assert.Equal(t, "", cr.GetItems()[0].GetSubject().GetRelation())
}
