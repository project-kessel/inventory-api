package tuples

import (
	"testing"

	pb "github.com/project-kessel/inventory-api/api/kessel/inventory/v1beta2"
	"github.com/project-kessel/inventory-api/internal/biz/model"
	tuplesctl "github.com/project-kessel/inventory-api/internal/biz/usecase/tuples"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestToCreateTuplesCommand(t *testing.T) {
	t.Run("valid request", func(t *testing.T) {
		req := &pb.CreateTuplesRequest{
			Tuples: []*pb.Relationship{
				{
					Resource: &pb.RelationObjectReference{
						Type: &pb.RelationObjectType{
							Namespace: "rbac",
							Name:      "workspace",
						},
						Id: "ws-1",
					},
					Relation: "member",
					Subject: &pb.RelationSubjectReference{
						Subject: &pb.RelationObjectReference{
							Type: &pb.RelationObjectType{
								Namespace: "rbac",
								Name:      "principal",
							},
							Id: "user-1",
						},
					},
				},
			},
			Upsert: true,
		}

		cmd, err := toCreateTuplesCommand(req)

		require.NoError(t, err)
		assert.True(t, cmd.Upsert)
		assert.Len(t, cmd.Tuples, 1)
	})

	t.Run("with fencing check", func(t *testing.T) {
		req := &pb.CreateTuplesRequest{
			Tuples: []*pb.Relationship{},
			FencingCheck: &pb.RelationFencingCheck{
				LockId:    "lock-1",
				LockToken: "token-1",
			},
		}

		cmd, err := toCreateTuplesCommand(req)

		require.NoError(t, err)
		require.NotNil(t, cmd.FencingCheck)
		assert.Equal(t, "lock-1", cmd.FencingCheck.LockId)
		assert.Equal(t, "token-1", cmd.FencingCheck.LockToken)
	})

	t.Run("invalid relationship - bad resource ID", func(t *testing.T) {
		req := &pb.CreateTuplesRequest{
			Tuples: []*pb.Relationship{
				{
					Resource: &pb.RelationObjectReference{
						Type: &pb.RelationObjectType{
							Namespace: "rbac",
							Name:      "workspace",
						},
						Id: "", // invalid
					},
					Relation: "member",
					Subject: &pb.RelationSubjectReference{
						Subject: &pb.RelationObjectReference{
							Type: &pb.RelationObjectType{
								Namespace: "rbac",
								Name:      "principal",
							},
							Id: "user-1",
						},
					},
				},
			},
		}

		_, err := toCreateTuplesCommand(req)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid relationship at index 0")
	})
}

func TestToDeleteTuplesCommand(t *testing.T) {
	t.Run("with filter", func(t *testing.T) {
		namespace := "rbac"
		resourceType := "workspace"

		req := &pb.DeleteTuplesRequest{
			Filter: &pb.RelationTupleFilter{
				ResourceNamespace: &namespace,
				ResourceType:      &resourceType,
			},
		}

		cmd, err := toDeleteTuplesCommand(req)

		require.NoError(t, err)
		assert.Equal(t, &namespace, cmd.Filter.ResourceNamespace)
		assert.Equal(t, &resourceType, cmd.Filter.ResourceType)
	})

	t.Run("with fencing check", func(t *testing.T) {
		req := &pb.DeleteTuplesRequest{
			Filter: &pb.RelationTupleFilter{},
			FencingCheck: &pb.RelationFencingCheck{
				LockId:    "lock-2",
				LockToken: "token-2",
			},
		}

		cmd, err := toDeleteTuplesCommand(req)

		require.NoError(t, err)
		require.NotNil(t, cmd.FencingCheck)
		assert.Equal(t, "lock-2", cmd.FencingCheck.LockId)
	})
}

func TestToReadTuplesCommand(t *testing.T) {
	t.Run("with pagination", func(t *testing.T) {
		token := "token-xyz"
		req := &pb.ReadTuplesRequest{
			Filter: &pb.RelationTupleFilter{},
			Pagination: &pb.RequestPagination{
				Limit:             10,
				ContinuationToken: &token,
			},
		}

		cmd, err := toReadTuplesCommand(req)

		require.NoError(t, err)
		require.NotNil(t, cmd.Pagination)
		assert.Equal(t, uint32(10), cmd.Pagination.Limit)
		assert.Equal(t, &token, cmd.Pagination.Continuation)
	})

	t.Run("with consistency", func(t *testing.T) {
		req := &pb.ReadTuplesRequest{
			Filter: &pb.RelationTupleFilter{},
			Consistency: &pb.Consistency{
				Requirement: &pb.Consistency_MinimizeLatency{MinimizeLatency: true},
			},
		}

		cmd, err := toReadTuplesCommand(req)

		require.NoError(t, err)
		assert.Equal(t, model.ConsistencyMinimizeLatency, model.ConsistencyTypeOf(cmd.Consistency))
	})
}

func TestToAcquireLockCommand(t *testing.T) {
	req := &pb.AcquireLockRequest{
		LockId: "lock-123",
	}

	cmd := toAcquireLockCommand(req)

	assert.Equal(t, "lock-123", cmd.LockId)
}

func TestRelationshipToRelationsTuple(t *testing.T) {
	t.Run("valid relationship", func(t *testing.T) {
		rel := &pb.Relationship{
			Resource: &pb.RelationObjectReference{
				Type: &pb.RelationObjectType{
					Namespace: "rbac",
					Name:      "workspace",
				},
				Id: "ws-1",
			},
			Relation: "member",
			Subject: &pb.RelationSubjectReference{
				Subject: &pb.RelationObjectReference{
					Type: &pb.RelationObjectType{
						Namespace: "rbac",
						Name:      "principal",
					},
					Id: "user-1",
				},
			},
		}

		tuple, err := relationshipToRelationsTuple(rel)

		require.NoError(t, err)
		assert.Equal(t, "ws-1", tuple.Resource().Id().Serialize())
		assert.Equal(t, "workspace", tuple.Resource().Type().Name())
		assert.Equal(t, "rbac", tuple.Resource().Type().Namespace())
		assert.Equal(t, model.DeserializeRelation("member"), tuple.Relation())
		assert.Equal(t, "user-1", tuple.Subject().Subject().Id().Serialize())
	})

	t.Run("with subject relation", func(t *testing.T) {
		subjectRelation := "members"
		rel := &pb.Relationship{
			Resource: &pb.RelationObjectReference{
				Type: &pb.RelationObjectType{
					Namespace: "rbac",
					Name:      "workspace",
				},
				Id: "ws-1",
			},
			Relation: "member",
			Subject: &pb.RelationSubjectReference{
				Relation: &subjectRelation,
				Subject: &pb.RelationObjectReference{
					Type: &pb.RelationObjectType{
						Namespace: "rbac",
						Name:      "group",
					},
					Id: "group-1",
				},
			},
		}

		tuple, err := relationshipToRelationsTuple(rel)

		require.NoError(t, err)
		expectedRel := model.DeserializeRelation("members")
		assert.Equal(t, &expectedRel, tuple.Subject().Relation())
	})

	t.Run("invalid resource ID", func(t *testing.T) {
		rel := &pb.Relationship{
			Resource: &pb.RelationObjectReference{
				Type: &pb.RelationObjectType{
					Namespace: "rbac",
					Name:      "workspace",
				},
				Id: "", // invalid
			},
			Relation: "member",
			Subject: &pb.RelationSubjectReference{
				Subject: &pb.RelationObjectReference{
					Type: &pb.RelationObjectType{
						Namespace: "rbac",
						Name:      "principal",
					},
					Id: "user-1",
				},
			},
		}

		_, err := relationshipToRelationsTuple(rel)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid resource ID")
	})
}

func TestTupleFilterFromProto(t *testing.T) {
	t.Run("nil filter", func(t *testing.T) {
		result := tupleFilterFromProto(nil)
		assert.NotNil(t, result)
	})

	t.Run("with all fields", func(t *testing.T) {
		namespace := "rbac"
		resourceType := "workspace"
		resourceId := "ws-1"
		relation := "member"

		pf := &pb.RelationTupleFilter{
			ResourceNamespace: &namespace,
			ResourceType:      &resourceType,
			ResourceId:        &resourceId,
			Relation:          &relation,
		}

		result := tupleFilterFromProto(pf)

		assert.Equal(t, &namespace, result.ResourceNamespace)
		assert.Equal(t, &resourceType, result.ResourceType)
		assert.Equal(t, &resourceId, result.ResourceId)
		assert.Equal(t, &relation, result.Relation)
	})

	t.Run("with subject filter", func(t *testing.T) {
		subjectNamespace := "rbac"
		subjectType := "principal"

		pf := &pb.RelationTupleFilter{
			SubjectFilter: &pb.RelationSubjectFilter{
				SubjectNamespace: &subjectNamespace,
				SubjectType:      &subjectType,
			},
		}

		result := tupleFilterFromProto(pf)

		require.NotNil(t, result.SubjectFilter)
		assert.Equal(t, &subjectNamespace, result.SubjectFilter.SubjectNamespace)
		assert.Equal(t, &subjectType, result.SubjectFilter.SubjectType)
	})
}

func TestPaginationFromProto(t *testing.T) {
	t.Run("nil pagination", func(t *testing.T) {
		result := paginationFromProto(nil)
		assert.Nil(t, result)
	})

	t.Run("with continuation token", func(t *testing.T) {
		token := "token-123"
		p := &pb.RequestPagination{
			Limit:             25,
			ContinuationToken: &token,
		}

		result := paginationFromProto(p)

		require.NotNil(t, result)
		assert.Equal(t, uint32(25), result.Limit)
		require.NotNil(t, result.Continuation)
		assert.Equal(t, token, *result.Continuation)
	})

	t.Run("without continuation token", func(t *testing.T) {
		p := &pb.RequestPagination{
			Limit: 50,
		}

		result := paginationFromProto(p)

		require.NotNil(t, result)
		assert.Equal(t, uint32(50), result.Limit)
		assert.Nil(t, result.Continuation)
	})
}

func TestConsistencyFromProto(t *testing.T) {
	t.Run("nil consistency returns unspecified", func(t *testing.T) {
		result := consistencyFromProto(nil)
		assert.Equal(t, model.ConsistencyUnspecified, model.ConsistencyTypeOf(result))
	})

	t.Run("minimize latency", func(t *testing.T) {
		c := &pb.Consistency{
			Requirement: &pb.Consistency_MinimizeLatency{MinimizeLatency: true},
		}

		result := consistencyFromProto(c)

		assert.Equal(t, model.ConsistencyMinimizeLatency, model.ConsistencyTypeOf(result))
	})

	t.Run("at least as acknowledged", func(t *testing.T) {
		c := &pb.Consistency{
			Requirement: &pb.Consistency_AtLeastAsAcknowledged{AtLeastAsAcknowledged: true},
		}

		result := consistencyFromProto(c)

		assert.Equal(t, model.ConsistencyAtLeastAsAcknowledged, model.ConsistencyTypeOf(result))
	})

	t.Run("at least as fresh", func(t *testing.T) {
		c := &pb.Consistency{
			Requirement: &pb.Consistency_AtLeastAsFresh{
				AtLeastAsFresh: &pb.ConsistencyToken{Token: "token-xyz"},
			},
		}

		result := consistencyFromProto(c)

		assert.Equal(t, model.ConsistencyAtLeastAsFresh, model.ConsistencyTypeOf(result))
		fresh, ok := model.AsAtLeastAsFresh(result)
		require.True(t, ok)
		assert.Equal(t, "token-xyz", fresh.ConsistencyToken().Serialize())
	})
}

func TestFromCreateTuplesResult(t *testing.T) {
	t.Run("with consistency token", func(t *testing.T) {
		result := &tuplesctl.CreateTuplesResult{
			ConsistencyToken: model.DeserializeConsistencyToken("token-123"),
		}

		resp := fromCreateTuplesResult(result)

		require.NotNil(t, resp)
		require.NotNil(t, resp.ConsistencyToken)
		assert.Equal(t, "token-123", resp.ConsistencyToken.Token)
	})

	t.Run("without consistency token", func(t *testing.T) {
		result := &tuplesctl.CreateTuplesResult{
			ConsistencyToken: "",
		}

		resp := fromCreateTuplesResult(result)

		require.NotNil(t, resp)
		assert.Nil(t, resp.ConsistencyToken)
	})
}

func TestFromDeleteTuplesResult(t *testing.T) {
	t.Run("with consistency token", func(t *testing.T) {
		result := &tuplesctl.DeleteTuplesResult{
			ConsistencyToken: model.DeserializeConsistencyToken("token-456"),
		}

		resp := fromDeleteTuplesResult(result)

		require.NotNil(t, resp)
		require.NotNil(t, resp.ConsistencyToken)
		assert.Equal(t, "token-456", resp.ConsistencyToken.Token)
	})

	t.Run("without consistency token", func(t *testing.T) {
		result := &tuplesctl.DeleteTuplesResult{
			ConsistencyToken: "",
		}

		resp := fromDeleteTuplesResult(result)

		require.NotNil(t, resp)
		assert.Nil(t, resp.ConsistencyToken)
	})
}

func TestReadTuplesItemToProto(t *testing.T) {
	t.Run("basic item", func(t *testing.T) {
		item := model.ReadTuplesItem{
			ResourceNamespace: "rbac",
			ResourceType:      "workspace",
			ResourceId:        "ws-1",
			Relation:          "member",
			SubjectNamespace:  "rbac",
			SubjectType:       "principal",
			SubjectId:         "user-1",
		}

		result := readTuplesItemToProto(item)

		require.NotNil(t, result.Tuple)
		assert.Equal(t, "rbac", result.Tuple.Resource.Type.Namespace)
		assert.Equal(t, "workspace", result.Tuple.Resource.Type.Name)
		assert.Equal(t, "ws-1", result.Tuple.Resource.Id)
		assert.Equal(t, "member", result.Tuple.Relation)
		assert.Equal(t, "user-1", result.Tuple.Subject.Subject.Id)
		assert.Nil(t, result.Pagination)
		assert.Nil(t, result.ConsistencyToken)
	})

	t.Run("with subject relation", func(t *testing.T) {
		subjectRelation := "members"
		item := model.ReadTuplesItem{
			ResourceNamespace: "rbac",
			ResourceType:      "workspace",
			ResourceId:        "ws-1",
			Relation:          "member",
			SubjectNamespace:  "rbac",
			SubjectType:       "group",
			SubjectId:         "group-1",
			SubjectRelation:   &subjectRelation,
		}

		result := readTuplesItemToProto(item)

		require.NotNil(t, result.Tuple.Subject.Relation)
		assert.Equal(t, "members", *result.Tuple.Subject.Relation)
	})

	t.Run("with pagination and consistency token", func(t *testing.T) {
		item := model.ReadTuplesItem{
			ResourceNamespace: "rbac",
			ResourceType:      "workspace",
			ResourceId:        "ws-1",
			Relation:          "member",
			SubjectNamespace:  "rbac",
			SubjectType:       "principal",
			SubjectId:         "user-1",
			ContinuationToken: "page-token-abc",
			ConsistencyToken:  model.DeserializeConsistencyToken("ct-123"),
		}

		result := readTuplesItemToProto(item)

		require.NotNil(t, result.Pagination)
		assert.Equal(t, "page-token-abc", result.Pagination.ContinuationToken)
		require.NotNil(t, result.ConsistencyToken)
		assert.Equal(t, "ct-123", result.ConsistencyToken.Token)
	})
}
