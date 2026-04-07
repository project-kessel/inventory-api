package v1beta2

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/genproto/googleapis/rpc/status"
	"google.golang.org/grpc/codes"
	"google.golang.org/protobuf/proto"
)

func TestCheckBulkResponse_FullRoundTrip(t *testing.T) {
	relation := "members"
	resp := &CheckBulkResponse{
		Pairs: []*CheckBulkResponsePair{
			{
				Request: &CheckBulkRequestItem{
					Object:   &ResourceReference{ResourceType: "host", ResourceId: "host-123"},
					Relation: "view",
					Subject:  &SubjectReference{Relation: &relation, Resource: &ResourceReference{ResourceType: "principal", ResourceId: "sarah"}},
				},
				Response: &CheckBulkResponsePair_Item{
					Item: &CheckBulkResponseItem{
						Allowed: Allowed_ALLOWED_TRUE,
					},
				},
			},
		},
		ConsistencyToken: &ConsistencyToken{Token: "test-token"},
	}

	// Marshal to protobuf and back
	data, err := proto.Marshal(resp)
	assert.NoError(t, err)

	var out CheckBulkResponse
	err = proto.Unmarshal(data, &out)
	assert.NoError(t, err)

	// Check that we have one pair
	assert.Len(t, out.GetPairs(), 1)

	pair := out.GetPairs()[0]

	// Check request
	assert.NotNil(t, pair.GetRequest())
	assert.Equal(t, "host", pair.GetRequest().GetObject().GetResourceType())
	assert.Equal(t, "host-123", pair.GetRequest().GetObject().GetResourceId())
	assert.Equal(t, "view", pair.GetRequest().GetRelation())
	assert.NotNil(t, pair.GetRequest().GetSubject())
	assert.Equal(t, "members", pair.GetRequest().GetSubject().GetRelation())

	// Check response item
	assert.NotNil(t, pair.GetItem())
	assert.Equal(t, Allowed_ALLOWED_TRUE, pair.GetItem().GetAllowed())
	assert.Nil(t, pair.GetError())

	// Check consistency token
	assert.NotNil(t, out.GetConsistencyToken())
	assert.Equal(t, "test-token", out.GetConsistencyToken().GetToken())
}

func TestCheckBulkResponse_WithError(t *testing.T) {
	resp := &CheckBulkResponse{
		Pairs: []*CheckBulkResponsePair{
			{
				Request: &CheckBulkRequestItem{
					Object:   &ResourceReference{ResourceType: "host", ResourceId: "host-456"},
					Relation: "delete",
				},
				Response: &CheckBulkResponsePair_Error{
					Error: &status.Status{
						Code:    int32(codes.NotFound),
						Message: "Not found",
					},
				},
			},
		},
	}

	// Marshal to protobuf and back
	data, err := proto.Marshal(resp)
	assert.NoError(t, err)

	var out CheckBulkResponse
	err = proto.Unmarshal(data, &out)
	assert.NoError(t, err)

	// Check that we have one pair
	assert.Len(t, out.GetPairs(), 1)

	pair := out.GetPairs()[0]

	// Check error response
	assert.Nil(t, pair.GetItem())
	assert.NotNil(t, pair.GetError())
	assert.Equal(t, int32(codes.NotFound), pair.GetError().GetCode())
	assert.Equal(t, "Not found", pair.GetError().GetMessage())
}

func TestCheckBulkResponse_MultipleResults(t *testing.T) {
	resp := &CheckBulkResponse{
		Pairs: []*CheckBulkResponsePair{
			{
				Request: &CheckBulkRequestItem{
					Object:   &ResourceReference{ResourceType: "host", ResourceId: "host-1"},
					Relation: "view",
				},
				Response: &CheckBulkResponsePair_Item{
					Item: &CheckBulkResponseItem{Allowed: Allowed_ALLOWED_TRUE},
				},
			},
			{
				Request: &CheckBulkRequestItem{
					Object:   &ResourceReference{ResourceType: "host", ResourceId: "host-2"},
					Relation: "delete",
				},
				Response: &CheckBulkResponsePair_Item{
					Item: &CheckBulkResponseItem{Allowed: Allowed_ALLOWED_FALSE},
				},
			},
		},
	}

	data, err := proto.Marshal(resp)
	require.NoError(t, err)

	var decoded CheckBulkResponse
	err = proto.Unmarshal(data, &decoded)
	require.NoError(t, err)

	require.Len(t, decoded.GetPairs(), 2)

	assert.Equal(t, Allowed_ALLOWED_TRUE, decoded.GetPairs()[0].GetItem().GetAllowed())
	assert.Equal(t, "host-1", decoded.GetPairs()[0].GetRequest().GetObject().GetResourceId())

	assert.Equal(t, Allowed_ALLOWED_FALSE, decoded.GetPairs()[1].GetItem().GetAllowed())
	assert.Equal(t, "host-2", decoded.GetPairs()[1].GetRequest().GetObject().GetResourceId())
}

func TestCheckBulkResponse_BasicBehavior(t *testing.T) {
	t.Run("nil pointer safety", func(t *testing.T) {
		var resp *CheckBulkResponse
		assert.Nil(t, resp.GetPairs())
		assert.Nil(t, resp.GetConsistencyToken())
	})
}

func TestCheckBulkResponsePair_BasicBehavior(t *testing.T) {
	t.Run("nil pointer safety", func(t *testing.T) {
		var pair *CheckBulkResponsePair
		assert.Nil(t, pair.GetRequest())
		assert.Nil(t, pair.GetItem())
		assert.Nil(t, pair.GetError())
	})
}

func TestCheckBulkResponseItem_BasicBehavior(t *testing.T) {
	t.Run("nil pointer safety", func(t *testing.T) {
		var item *CheckBulkResponseItem
		assert.Equal(t, Allowed_ALLOWED_UNSPECIFIED, item.GetAllowed())
	})
}

// TestCheckBulkResponse_MixedItemsAndErrors verifies that a response containing both Item
// and Error pairs round-trips correctly, that oneof exclusivity is preserved, and that the
// ConsistencyToken survives serialization.
func TestCheckBulkResponse_MixedItemsAndErrors(t *testing.T) {
	relation := "members"
	resp := &CheckBulkResponse{
		Pairs: []*CheckBulkResponsePair{
			{
				Request: &CheckBulkRequestItem{
					Object:   &ResourceReference{ResourceType: "host", ResourceId: "host-1"},
					Relation: "view",
					Subject: &SubjectReference{
						Relation: &relation,
						Resource: &ResourceReference{ResourceType: "principal", ResourceId: "alice"},
					},
				},
				Response: &CheckBulkResponsePair_Item{
					Item: &CheckBulkResponseItem{Allowed: Allowed_ALLOWED_TRUE},
				},
			},
			{
				Request: &CheckBulkRequestItem{
					Object:   &ResourceReference{ResourceType: "host", ResourceId: "host-2"},
					Relation: "delete",
				},
				Response: &CheckBulkResponsePair_Error{
					Error: &status.Status{
						Code:    int32(codes.PermissionDenied),
						Message: "permission denied",
					},
				},
			},
		},
		ConsistencyToken: &ConsistencyToken{Token: "consistency-xyz"},
	}

	data, err := proto.Marshal(resp)
	require.NoError(t, err)

	var decoded CheckBulkResponse
	err = proto.Unmarshal(data, &decoded)
	require.NoError(t, err)

	require.NotNil(t, decoded.GetConsistencyToken())
	assert.Equal(t, "consistency-xyz", decoded.GetConsistencyToken().GetToken())

	require.Len(t, decoded.GetPairs(), 2)

	// First pair: Item response — Error must be nil (oneof exclusivity).
	pair0 := decoded.GetPairs()[0]
	assert.Equal(t, "host-1", pair0.GetRequest().GetObject().GetResourceId())
	assert.NotNil(t, pair0.GetItem(), "first pair should carry an Item response")
	assert.Nil(t, pair0.GetError(), "first pair must not carry an Error (oneof)")
	assert.Equal(t, Allowed_ALLOWED_TRUE, pair0.GetItem().GetAllowed())

	// Second pair: Error response — Item must be nil (oneof exclusivity).
	pair1 := decoded.GetPairs()[1]
	assert.Equal(t, "host-2", pair1.GetRequest().GetObject().GetResourceId())
	assert.Nil(t, pair1.GetItem(), "second pair must not carry an Item (oneof)")
	assert.NotNil(t, pair1.GetError(), "second pair should carry an Error response")
	assert.Equal(t, int32(codes.PermissionDenied), pair1.GetError().GetCode())
	assert.Equal(t, "permission denied", pair1.GetError().GetMessage())
}

func TestCheckBulkResponseItem_AllowedValues(t *testing.T) {
	testCases := []struct {
		name    string
		allowed Allowed
	}{
		{"Unspecified", Allowed_ALLOWED_UNSPECIFIED},
		{"True", Allowed_ALLOWED_TRUE},
		{"False", Allowed_ALLOWED_FALSE},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			item := &CheckBulkResponseItem{
				Allowed: tc.allowed,
			}
			assert.Equal(t, tc.allowed, item.GetAllowed())
		})
	}
}
