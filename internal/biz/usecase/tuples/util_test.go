package tuples

import (
	"testing"

	"github.com/project-kessel/inventory-api/internal/biz/model"
	"github.com/stretchr/testify/assert"
)

func TestCreateTuplesCommandToV1beta1(t *testing.T) {
	resourceId, _ := model.NewLocalResourceId("resource-1")
	subjectId, _ := model.NewLocalResourceId("subject-1")

	tuple := model.NewRelationsTuple(
		model.NewRelationsResource(resourceId, model.NewRelationsObjectType("workspace", "rbac")),
		"member",
		model.NewRelationsSubject(
			model.NewRelationsResource(subjectId, model.NewRelationsObjectType("principal", "rbac")),
			"",
		),
	)

	t.Run("with fencing check", func(t *testing.T) {
		cmd := CreateTuplesCommand{
			Tuples: []model.RelationsTuple{tuple},
			Upsert: true,
			FencingCheck: &FencingCheck{
				LockId:    "lock-1",
				LockToken: "token-1",
			},
		}

		req := createTuplesCommandToV1beta1(cmd)

		assert.NotNil(t, req)
		assert.True(t, req.Upsert)
		assert.Len(t, req.Tuples, 1)
		assert.NotNil(t, req.FencingCheck)
		assert.Equal(t, "lock-1", req.FencingCheck.LockId)
		assert.Equal(t, "token-1", req.FencingCheck.LockToken)
	})

	t.Run("without fencing check", func(t *testing.T) {
		cmd := CreateTuplesCommand{
			Tuples: []model.RelationsTuple{tuple},
			Upsert: false,
		}

		req := createTuplesCommandToV1beta1(cmd)

		assert.NotNil(t, req)
		assert.False(t, req.Upsert)
		assert.Len(t, req.Tuples, 1)
		assert.Nil(t, req.FencingCheck)
	})
}

func TestDeleteTuplesCommandToV1beta1(t *testing.T) {
	t.Run("with all filter fields", func(t *testing.T) {
		namespace := "rbac"
		resourceType := "workspace"
		resourceId := "ws-1"
		relation := "member"

		cmd := DeleteTuplesCommand{
			Filter: TupleFilter{
				ResourceNamespace: &namespace,
				ResourceType:      &resourceType,
				ResourceId:        &resourceId,
				Relation:          &relation,
			},
		}

		req := deleteTuplesCommandToV1beta1(cmd)

		assert.NotNil(t, req)
		assert.NotNil(t, req.Filter)
		assert.Equal(t, &namespace, req.Filter.ResourceNamespace)
		assert.Equal(t, &resourceType, req.Filter.ResourceType)
		assert.Equal(t, &resourceId, req.Filter.ResourceId)
		assert.Equal(t, &relation, req.Filter.Relation)
	})

	t.Run("with subject filter", func(t *testing.T) {
		subjectNamespace := "rbac"
		subjectType := "principal"

		cmd := DeleteTuplesCommand{
			Filter: TupleFilter{
				SubjectFilter: &SubjectFilter{
					SubjectNamespace: &subjectNamespace,
					SubjectType:      &subjectType,
				},
			},
		}

		req := deleteTuplesCommandToV1beta1(cmd)

		assert.NotNil(t, req.Filter.SubjectFilter)
		assert.Equal(t, &subjectNamespace, req.Filter.SubjectFilter.SubjectNamespace)
		assert.Equal(t, &subjectType, req.Filter.SubjectFilter.SubjectType)
	})
}

func TestReadTuplesCommandToV1beta1(t *testing.T) {
	t.Run("with pagination", func(t *testing.T) {
		token := "token-123"
		cmd := ReadTuplesCommand{
			Filter: TupleFilter{},
			Pagination: &model.Pagination{
				Limit:        10,
				Continuation: &token,
			},
			Consistency: model.NewConsistencyMinimizeLatency(),
		}

		req := readTuplesCommandToV1beta1(cmd)

		assert.NotNil(t, req)
		assert.NotNil(t, req.Pagination)
		assert.Equal(t, uint32(10), req.Pagination.Limit)
		assert.Equal(t, &token, req.Pagination.ContinuationToken)
	})

	t.Run("without pagination", func(t *testing.T) {
		cmd := ReadTuplesCommand{
			Filter:      TupleFilter{},
			Consistency: model.NewConsistencyMinimizeLatency(),
		}

		req := readTuplesCommandToV1beta1(cmd)

		assert.NotNil(t, req)
		assert.Nil(t, req.Pagination)
	})
}

func TestAcquireLockCommandToV1beta1(t *testing.T) {
	cmd := AcquireLockCommand{
		LockId: "lock-123",
	}

	req := acquireLockCommandToV1beta1(cmd)

	assert.NotNil(t, req)
	assert.Equal(t, "lock-123", req.LockId)
}

func TestRelationsTupleToV1beta1(t *testing.T) {
	t.Run("with subject relation", func(t *testing.T) {
		resourceId, _ := model.NewLocalResourceId("ws-1")
		subjectId, _ := model.NewLocalResourceId("group-1")

		tuple := model.NewRelationsTuple(
			model.NewRelationsResource(resourceId, model.NewRelationsObjectType("workspace", "rbac")),
			"member",
			model.NewRelationsSubject(
				model.NewRelationsResource(subjectId, model.NewRelationsObjectType("group", "rbac")),
				"members",
			),
		)

		rel := relationsTupleToV1beta1(tuple)

		assert.NotNil(t, rel)
		assert.Equal(t, "rbac", rel.Resource.Type.Namespace)
		assert.Equal(t, "workspace", rel.Resource.Type.Name)
		assert.Equal(t, "ws-1", rel.Resource.Id)
		assert.Equal(t, "member", rel.Relation)
		assert.Equal(t, "rbac", rel.Subject.Subject.Type.Namespace)
		assert.Equal(t, "group", rel.Subject.Subject.Type.Name)
		assert.Equal(t, "group-1", rel.Subject.Subject.Id)
		assert.NotNil(t, rel.Subject.Relation)
		assert.Equal(t, "members", *rel.Subject.Relation)
	})

	t.Run("without subject relation", func(t *testing.T) {
		resourceId, _ := model.NewLocalResourceId("ws-1")
		subjectId, _ := model.NewLocalResourceId("user-1")

		tuple := model.NewRelationsTuple(
			model.NewRelationsResource(resourceId, model.NewRelationsObjectType("workspace", "rbac")),
			"member",
			model.NewRelationsSubject(
				model.NewRelationsResource(subjectId, model.NewRelationsObjectType("principal", "rbac")),
				"",
			),
		)

		rel := relationsTupleToV1beta1(tuple)

		assert.NotNil(t, rel)
		assert.Equal(t, "user-1", rel.Subject.Subject.Id)
		assert.Nil(t, rel.Subject.Relation)
	})
}

func TestConsistencyToV1beta1(t *testing.T) {
	t.Run("minimize latency", func(t *testing.T) {
		c := model.NewConsistencyMinimizeLatency()

		result := consistencyToV1beta1(c)

		assert.NotNil(t, result)
		assert.NotNil(t, result.GetMinimizeLatency())
		assert.True(t, result.GetMinimizeLatency())
	})

	t.Run("at least as fresh", func(t *testing.T) {
		token := model.DeserializeConsistencyToken("token-123")
		c := model.NewConsistencyAtLeastAsFresh(token)

		result := consistencyToV1beta1(c)

		assert.NotNil(t, result)
		assert.NotNil(t, result.GetAtLeastAsFresh())
		assert.Equal(t, "token-123", result.GetAtLeastAsFresh().Token)
	})

	t.Run("at least as acknowledged maps to minimize latency", func(t *testing.T) {
		c := model.NewConsistencyAtLeastAsAcknowledged()

		result := consistencyToV1beta1(c)

		assert.NotNil(t, result)
		assert.NotNil(t, result.GetMinimizeLatency())
		assert.True(t, result.GetMinimizeLatency())
	})

	t.Run("unspecified maps to minimize latency", func(t *testing.T) {
		c := model.NewConsistencyUnspecified()

		result := consistencyToV1beta1(c)

		assert.NotNil(t, result)
		assert.NotNil(t, result.GetMinimizeLatency())
		assert.True(t, result.GetMinimizeLatency())
	})
}

func TestPaginationToV1beta1(t *testing.T) {
	t.Run("nil pagination", func(t *testing.T) {
		result := paginationToV1beta1(nil)
		assert.Nil(t, result)
	})

	t.Run("with continuation token", func(t *testing.T) {
		token := "token-xyz"
		pagination := &model.Pagination{
			Limit:        25,
			Continuation: &token,
		}

		result := paginationToV1beta1(pagination)

		assert.NotNil(t, result)
		assert.Equal(t, uint32(25), result.Limit)
		assert.NotNil(t, result.ContinuationToken)
		assert.Equal(t, &token, result.ContinuationToken)
	})

	t.Run("without continuation token", func(t *testing.T) {
		pagination := &model.Pagination{
			Limit: 50,
		}

		result := paginationToV1beta1(pagination)

		assert.NotNil(t, result)
		assert.Equal(t, uint32(50), result.Limit)
		assert.Nil(t, result.ContinuationToken)
	})
}

func TestTupleFilterToV1beta1(t *testing.T) {
	namespace := "rbac"
	resourceType := "workspace"

	filter := TupleFilter{
		ResourceNamespace: &namespace,
		ResourceType:      &resourceType,
	}

	result := tupleFilterToV1beta1(filter)

	assert.NotNil(t, result)
	assert.Equal(t, &namespace, result.ResourceNamespace)
	assert.Equal(t, &resourceType, result.ResourceType)
}

func TestFencingCheckToV1beta1(t *testing.T) {
	t.Run("nil fencing check", func(t *testing.T) {
		result := fencingCheckToV1beta1(nil)
		assert.Nil(t, result)
	})

	t.Run("valid fencing check", func(t *testing.T) {
		fc := &FencingCheck{
			LockId:    "lock-1",
			LockToken: "token-abc",
		}

		result := fencingCheckToV1beta1(fc)

		assert.NotNil(t, result)
		assert.Equal(t, "lock-1", result.LockId)
		assert.Equal(t, "token-abc", result.LockToken)
	})
}

func TestSubjectFilterToV1beta1(t *testing.T) {
	t.Run("nil subject filter", func(t *testing.T) {
		result := subjectFilterToV1beta1(nil)
		assert.Nil(t, result)
	})

	t.Run("valid subject filter", func(t *testing.T) {
		namespace := "rbac"
		subjectType := "principal"

		filter := &SubjectFilter{
			SubjectNamespace: &namespace,
			SubjectType:      &subjectType,
		}

		result := subjectFilterToV1beta1(filter)

		assert.NotNil(t, result)
		assert.Equal(t, &namespace, result.SubjectNamespace)
		assert.Equal(t, &subjectType, result.SubjectType)
	})
}
