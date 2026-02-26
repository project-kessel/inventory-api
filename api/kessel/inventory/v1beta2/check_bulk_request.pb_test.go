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
				Subject: &SubjectReference{
					Relation: &relation,
					Resource: &ResourceReference{
						ResourceType: "principal",
						ResourceId:   "sarah",
					},
				},
			},
		},
	}

	t.Run("json roundtrip", func(t *testing.T) {
		data, err := json.Marshal(cr)
		assert.NoError(t, err)

		var decoded CheckBulkRequest
		err = json.Unmarshal(data, &decoded)
		assert.NoError(t, err)

		if assert.Len(t, decoded.Items, 1) {
			item := decoded.Items[0]
			if assert.NotNil(t, item.Object) {
				assert.Equal(t, "host", item.Object.ResourceType)
				assert.Equal(t, "host-123", item.Object.ResourceId)
			}
			assert.Equal(t, "view", item.Relation)

			if assert.NotNil(t, item.Subject) {
				if assert.NotNil(t, item.Subject.Relation) {
					assert.Equal(t, relation, *item.Subject.Relation)
				}
				if assert.NotNil(t, item.Subject.Resource) {
					assert.Equal(t, "principal", item.Subject.Resource.ResourceType)
					assert.Equal(t, "sarah", item.Subject.Resource.ResourceId)
				}
			}
		}
	})

	t.Run("protobuf roundtrip", func(t *testing.T) {
		data, err := proto.Marshal(cr)
		assert.NoError(t, err)

		var decoded CheckBulkRequest
		err = proto.Unmarshal(data, &decoded)
		assert.NoError(t, err)

		if assert.Len(t, decoded.Items, 1) {
			item := decoded.Items[0]
			if assert.NotNil(t, item.Object) {
				assert.Equal(t, "host", item.Object.ResourceType)
				assert.Equal(t, "host-123", item.Object.ResourceId)
			}
			assert.Equal(t, "view", item.Relation)

			if assert.NotNil(t, item.Subject) {
				if assert.NotNil(t, item.Subject.Relation) {
					assert.Equal(t, relation, *item.Subject.Relation)
				}
				if assert.NotNil(t, item.Subject.Resource) {
					assert.Equal(t, "principal", item.Subject.Resource.ResourceType)
					assert.Equal(t, "sarah", item.Subject.Resource.ResourceId)
				}
			}
		}
	})
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
func TestCheckBulkRequest_BasicBehavior(t *testing.T) {
	t.Run("string representation", func(t *testing.T) {
		cr := &CheckBulkRequest{
			Items: []*CheckBulkRequestItem{
				{
					Object:   &ResourceReference{ResourceType: "host"},
					Relation: "view",
				},
			},
		}
		s := cr.String()
		assert.NotEmpty(t, s, "String() should return non-empty representation")
	})

	t.Run("nil pointer safety", func(t *testing.T) {
		var cr *CheckBulkRequest
		// All getters should be safe to call on nil and return zero values
		assert.Nil(t, cr.GetItems())
		assert.Nil(t, cr.GetConsistency())
	})

	t.Run("empty struct", func(t *testing.T) {
		var cr CheckBulkRequest
		// All getters should return zero values, not panic
		assert.Nil(t, cr.GetItems())
		assert.Nil(t, cr.GetConsistency())
	})

	t.Run("subject with nil resource", func(t *testing.T) {
		cr := &CheckBulkRequest{
			Items: []*CheckBulkRequestItem{
				{
					Subject: &SubjectReference{Resource: nil},
				},
			},
		}
		assert.Nil(t, cr.GetItems()[0].GetSubject().GetResource())
		assert.Equal(t, "", cr.GetItems()[0].GetSubject().GetRelation())
	})
}

func TestCheckBulkRequest_ProtoInterface(t *testing.T) {
	// Verify protobuf interface implementation
	var cr interface{} = &CheckBulkRequest{}
	_, ok := cr.(proto.Message)
	assert.True(t, ok, "CheckBulkRequest should implement proto.Message")
}
