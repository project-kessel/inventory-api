package v1beta2

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/proto"
)

func TestCheckRequest_FullRoundTrip(t *testing.T) {
	relation := "members"
	cr := &CheckRequest{
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

	var out CheckRequest
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

func TestCheckRequest_Reset(t *testing.T) {
	cr := &CheckRequest{
		Object:   &ResourceReference{ResourceType: "vm", ResourceId: "123"},
		Relation: "read",
		Subject:  &SubjectReference{},
	}
	cr.Reset()
	assert.Nil(t, cr.Object)
	assert.Equal(t, "", cr.Relation)
	assert.Nil(t, cr.Subject)
}
func TestCheckRequest_String(t *testing.T) {
	cr := &CheckRequest{
		Object:   &ResourceReference{ResourceType: "host"},
		Relation: "view",
	}
	s := cr.String()
	assert.NotEmpty(t, s)
	// Optionally, check that it mentions your field values
	assert.Contains(t, s, "host")
	assert.Contains(t, s, "view")
}

func TestCheckRequest_ProtoMessage(t *testing.T) {
	var cr interface{} = &CheckRequest{}
	_, ok := cr.(proto.Message)
	assert.True(t, ok)
}

func TestCheckRequest_ProtoReflect(t *testing.T) {
	cr := &CheckRequest{
		Object:   &ResourceReference{ResourceType: "host"},
		Relation: "view",
	}
	m := cr.ProtoReflect()
	assert.NotNil(t, m)
	assert.Equal(t, "view", m.Interface().(*CheckRequest).Relation)
}

func TestCheckRequest_NilFields(t *testing.T) {
	var cr *CheckRequest
	// All getters should be safe to call on nil and return zero values
	assert.Nil(t, cr.GetObject())
	assert.Equal(t, "", cr.GetRelation())
	assert.Nil(t, cr.GetSubject())
}

func TestCheckRequest_EmptyStruct(t *testing.T) {
	var cr CheckRequest
	// All getters should return zero values, not panic
	assert.Nil(t, cr.GetObject())
	assert.Equal(t, "", cr.GetRelation())
	assert.Nil(t, cr.GetSubject())
}

func TestCheckRequest_SubjectNilResource(t *testing.T) {
	cr := &CheckRequest{
		Subject: &SubjectReference{
			Resource: nil,
		},
	}
	assert.Nil(t, cr.GetSubject().GetResource())
	assert.Equal(t, "", cr.GetSubject().GetRelation())
}

func TestCheckRequest_Consistency(t *testing.T) {
	// Test all consistency options
	tests := []struct {
		name        string
		consistency *Consistency
	}{
		{"nil", nil},
		{"minimize_latency", &Consistency{Requirement: &Consistency_MinimizeLatency{MinimizeLatency: true}}},
		{"at_least_as_fresh", &Consistency{Requirement: &Consistency_AtLeastAsFresh{AtLeastAsFresh: &ConsistencyToken{Token: "token"}}}},
		{"at_least_as_acknowledged", &Consistency{Requirement: &Consistency_AtLeastAsAcknowledged{AtLeastAsAcknowledged: true}}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cr := &CheckRequest{
				Object:      &ResourceReference{ResourceType: "host", ResourceId: "123"},
				Relation:    "view",
				Consistency: tc.consistency,
			}

			// Proto round-trip
			data, err := proto.Marshal(cr)
			assert.NoError(t, err)

			var out CheckRequest
			err = proto.Unmarshal(data, &out)
			assert.NoError(t, err)

			if tc.consistency == nil {
				assert.Nil(t, out.GetConsistency())
			} else {
				assert.NotNil(t, out.GetConsistency())
			}
		})
	}
}
