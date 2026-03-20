package v1beta2

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/encoding/protojson"
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
		data, err := protojson.Marshal(cr)
		require.NoError(t, err)

		var decoded CheckBulkRequest
		err = protojson.Unmarshal(data, &decoded)
		require.NoError(t, err)

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

func TestCheckBulkRequest_MultipleItems(t *testing.T) {
	cr := &CheckBulkRequest{
		Items: []*CheckBulkRequestItem{
			{Object: &ResourceReference{ResourceType: "host", ResourceId: "host-1"}, Relation: "view"},
			{Object: &ResourceReference{ResourceType: "host", ResourceId: "host-2"}, Relation: "edit"},
			{Object: &ResourceReference{ResourceType: "k8s_cluster", ResourceId: "cluster-1"}, Relation: "admin"},
		},
	}

	data, err := proto.Marshal(cr)
	require.NoError(t, err)

	var decoded CheckBulkRequest
	err = proto.Unmarshal(data, &decoded)
	require.NoError(t, err)

	require.Len(t, decoded.GetItems(), 3)
	assert.Equal(t, "host-1", decoded.GetItems()[0].GetObject().GetResourceId())
	assert.Equal(t, "view", decoded.GetItems()[0].GetRelation())
	assert.Equal(t, "host-2", decoded.GetItems()[1].GetObject().GetResourceId())
	assert.Equal(t, "edit", decoded.GetItems()[1].GetRelation())
	assert.Equal(t, "cluster-1", decoded.GetItems()[2].GetObject().GetResourceId())
	assert.Equal(t, "admin", decoded.GetItems()[2].GetRelation())
}

func TestCheckBulkRequest_OptionalRelationField(t *testing.T) {
	withRelation := "members"
	cr := &CheckBulkRequest{
		Items: []*CheckBulkRequestItem{
			{
				Object:   &ResourceReference{ResourceType: "host", ResourceId: "host-1"},
				Relation: "view",
				Subject: &SubjectReference{
					Relation: &withRelation,
					Resource: &ResourceReference{ResourceType: "principal", ResourceId: "alice"},
				},
			},
			{
				Object:   &ResourceReference{ResourceType: "host", ResourceId: "host-2"},
				Relation: "view",
				Subject: &SubjectReference{
					Resource: &ResourceReference{ResourceType: "principal", ResourceId: "bob"},
				},
			},
		},
	}

	data, err := proto.Marshal(cr)
	require.NoError(t, err)

	var decoded CheckBulkRequest
	err = proto.Unmarshal(data, &decoded)
	require.NoError(t, err)

	// First item: optional Relation should be preserved
	firstSubject := decoded.GetItems()[0].GetSubject()
	require.NotNil(t, firstSubject.Relation, "optional relation should be preserved through roundtrip")
	assert.Equal(t, "members", *firstSubject.Relation)

	// Second item: no Relation means the pointer remains nil
	secondSubject := decoded.GetItems()[1].GetSubject()
	assert.Nil(t, secondSubject.Relation, "unset optional relation should remain nil after roundtrip")
}
