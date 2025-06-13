package v1beta2

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/proto"
)

func TestCheckForUpdateRequest_FullRoundTrip(t *testing.T) {
	relation := "members"
	cr := &CheckForUpdateRequest{
		Object: &ResourceReference{
			ResourceType: "host",
			ResourceId:   "host-123",
		},
		Relation: "view",
		Subject: &SubjectReference{
			Relation: &relation,
			Resource: &ResourceReference{
				ResourceType: "principal",
				ResourceId:   "sarah",
			},
		},
	}

	// Marshal to JSON and back
	data, err := json.Marshal(cr)
	assert.NoError(t, err)

	var out CheckForUpdateRequest
	err = json.Unmarshal(data, &out)
	assert.NoError(t, err)

	// Check object
	assert.Equal(t, "host", out.GetObject().GetResourceType())
	assert.Equal(t, "host-123", out.GetObject().GetResourceId())

	// Check relation
	assert.Equal(t, "view", out.GetRelation())

	// Check subject
	assert.NotNil(t, out.GetSubject())
	assert.Equal(t, "members", out.GetSubject().GetRelation())
	assert.NotNil(t, out.GetSubject().GetResource())
	assert.Equal(t, "principal", out.GetSubject().GetResource().GetResourceType())
	assert.Equal(t, "sarah", out.GetSubject().GetResource().GetResourceId())
}

func TestCheckForUpdateRequest_Reset(t *testing.T) {
	cr := &CheckForUpdateRequest{
		Object:   &ResourceReference{ResourceType: "vm", ResourceId: "123"},
		Relation: "read",
		Subject:  &SubjectReference{},
	}
	cr.Reset()
	assert.Nil(t, cr.Object)
	assert.Equal(t, "", cr.Relation)
	assert.Nil(t, cr.Subject)
}
func TestCheckForUpdateRequest_String(t *testing.T) {
	cr := &CheckForUpdateRequest{
		Object:   &ResourceReference{ResourceType: "host"},
		Relation: "view",
	}
	s := cr.String()
	assert.NotEmpty(t, s)
	// Optionally, check that it mentions your field values
	assert.Contains(t, s, "host")
	assert.Contains(t, s, "view")
}

func TestCheckForUpdateRequest_ProtoMessage(t *testing.T) {
	var cr interface{} = &CheckForUpdateRequest{}
	_, ok := cr.(proto.Message)
	assert.True(t, ok)
}

func TestCheckForUpdateRequest_ProtoReflect(t *testing.T) {
	cr := &CheckForUpdateRequest{
		Object:   &ResourceReference{ResourceType: "host"},
		Relation: "view",
	}
	m := cr.ProtoReflect()
	assert.NotNil(t, m)
	assert.Equal(t, "view", m.Interface().(*CheckForUpdateRequest).Relation)
}

func TestCheckForUpdateRequest_NilFields(t *testing.T) {
	var cr *CheckForUpdateRequest
	// All getters should be safe to call on nil and return zero values
	assert.Nil(t, cr.GetObject())
	assert.Equal(t, "", cr.GetRelation())
	assert.Nil(t, cr.GetSubject())
}

func TestCheckForUpdateRequest_EmptyStruct(t *testing.T) {
	var cr CheckForUpdateRequest
	// All getters should return zero values, not panic
	assert.Nil(t, cr.GetObject())
	assert.Equal(t, "", cr.GetRelation())
	assert.Nil(t, cr.GetSubject())
}

func TestCheckForUpdateRequest_SubjectNilResource(t *testing.T) {
	cr := &CheckForUpdateRequest{
		Subject: &SubjectReference{
			Resource: nil,
		},
	}
	assert.Nil(t, cr.GetSubject().GetResource())
	assert.Equal(t, "", cr.GetSubject().GetRelation())
}
