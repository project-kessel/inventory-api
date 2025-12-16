package v1beta2

import (
	"testing"

	"github.com/stretchr/testify/assert"
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
					Item: &CheckBulkResponseItem{
						Allowed: Allowed_ALLOWED_TRUE,
					},
				},
			},
			{
				Request: &CheckBulkRequestItem{
					Object:   &ResourceReference{ResourceType: "host", ResourceId: "host-2"},
					Relation: "delete",
				},
				Response: &CheckBulkResponsePair_Item{
					Item: &CheckBulkResponseItem{
						Allowed: Allowed_ALLOWED_FALSE,
					},
				},
			},
		},
	}

	// Check that we have two pairs
	assert.Len(t, resp.GetPairs(), 2)

	// Check first result
	assert.Equal(t, Allowed_ALLOWED_TRUE, resp.GetPairs()[0].GetItem().GetAllowed())
	assert.Equal(t, "host-1", resp.GetPairs()[0].GetRequest().GetObject().GetResourceId())

	// Check second result
	assert.Equal(t, Allowed_ALLOWED_FALSE, resp.GetPairs()[1].GetItem().GetAllowed())
	assert.Equal(t, "host-2", resp.GetPairs()[1].GetRequest().GetObject().GetResourceId())
}

func TestCheckBulkResponse_BasicBehavior(t *testing.T) {
	t.Run("reset", func(t *testing.T) {
		resp := &CheckBulkResponse{
			Pairs: []*CheckBulkResponsePair{
				{
					Response: &CheckBulkResponsePair_Item{
						Item: &CheckBulkResponseItem{
							Allowed: Allowed_ALLOWED_TRUE,
						},
					},
				},
			},
			ConsistencyToken: &ConsistencyToken{Token: "test-token"},
		}
		resp.Reset()
		assert.Nil(t, resp.GetPairs())
		assert.Nil(t, resp.GetConsistencyToken())
	})

	t.Run("string representation", func(t *testing.T) {
		resp := &CheckBulkResponse{
			Pairs: []*CheckBulkResponsePair{
				{
					Request: &CheckBulkRequestItem{
						Object:   &ResourceReference{ResourceType: "host"},
						Relation: "view",
					},
				},
			},
		}
		s := resp.String()
		assert.NotEmpty(t, s, "String() should return non-empty representation")
	})

	t.Run("nil pointer safety", func(t *testing.T) {
		var resp *CheckBulkResponse
		// All getters should be safe to call on nil and return zero values
		assert.Nil(t, resp.GetPairs())
		assert.Nil(t, resp.GetConsistencyToken())
	})

	t.Run("empty struct", func(t *testing.T) {
		var resp CheckBulkResponse
		// All getters should return zero values, not panic
		assert.Nil(t, resp.GetPairs())
		assert.Nil(t, resp.GetConsistencyToken())
	})
}

func TestCheckBulkResponse_ProtoInterface(t *testing.T) {
	// Verify protobuf interface implementation
	var resp interface{} = &CheckBulkResponse{}
	_, ok := resp.(proto.Message)
	assert.True(t, ok, "CheckBulkResponse should implement proto.Message")
}

func TestCheckBulkResponsePair_BasicBehavior(t *testing.T) {
	t.Run("reset", func(t *testing.T) {
		pair := &CheckBulkResponsePair{
			Request: &CheckBulkRequestItem{
				Object:   &ResourceReference{ResourceType: "host", ResourceId: "123"},
				Relation: "view",
			},
			Response: &CheckBulkResponsePair_Item{
				Item: &CheckBulkResponseItem{
					Allowed: Allowed_ALLOWED_TRUE,
				},
			},
		}
		pair.Reset()
		assert.Nil(t, pair.GetRequest())
		assert.Nil(t, pair.GetItem())
		assert.Nil(t, pair.GetError())
	})

	t.Run("string representation", func(t *testing.T) {
		pair := &CheckBulkResponsePair{
			Request: &CheckBulkRequestItem{
				Object:   &ResourceReference{ResourceType: "vm"},
				Relation: "read",
			},
		}
		s := pair.String()
		assert.NotEmpty(t, s, "String() should return non-empty representation")
	})

	t.Run("nil pointer safety", func(t *testing.T) {
		var pair *CheckBulkResponsePair
		// All getters should be safe to call on nil and return zero values
		assert.Nil(t, pair.GetRequest())
		assert.Nil(t, pair.GetItem())
		assert.Nil(t, pair.GetError())
	})
}

func TestCheckBulkResponseItem_BasicBehavior(t *testing.T) {
	t.Run("reset", func(t *testing.T) {
		item := &CheckBulkResponseItem{
			Allowed: Allowed_ALLOWED_TRUE,
		}
		item.Reset()
		assert.Equal(t, Allowed_ALLOWED_UNSPECIFIED, item.GetAllowed())
	})

	t.Run("string representation", func(t *testing.T) {
		item := &CheckBulkResponseItem{
			Allowed: Allowed_ALLOWED_TRUE,
		}
		s := item.String()
		assert.NotEmpty(t, s, "String() should return non-empty representation")
	})

	t.Run("nil pointer safety", func(t *testing.T) {
		var item *CheckBulkResponseItem
		// Getter should be safe to call on nil and return zero value
		assert.Equal(t, Allowed_ALLOWED_UNSPECIFIED, item.GetAllowed())
	})
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
