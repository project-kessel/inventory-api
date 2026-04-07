package data

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/project-kessel/inventory-api/internal/biz/model"
)

func TestTupleToV1Beta1Relationship_PreservesOptionalSubjectRelation(t *testing.T) {
	rid, err := model.NewLocalResourceId("res-1")
	require.NoError(t, err)
	resource := model.NewRelationsResource(rid, model.NewRelationsObjectType("host", "insights"))

	sid, err := model.NewLocalResourceId("group-1")
	require.NoError(t, err)
	subjectRes := model.NewRelationsResource(sid, model.NewRelationsObjectType("group", "rbac"))
	subjectWithRel := model.NewRelationsSubject(subjectRes, "member")

	tuple := model.NewRelationsTuple(resource, "viewer", subjectWithRel)

	rel, err := tupleToV1Beta1Relationship(tuple)
	require.NoError(t, err)
	require.NotNil(t, rel.Subject)
	assert.Equal(t, "member", rel.Subject.GetRelation())

	filter, err := tupleToV1Beta1Filter(tuple)
	require.NoError(t, err)
	require.NotNil(t, filter.SubjectFilter)
	assert.Equal(t, "member", filter.SubjectFilter.GetRelation())
}

func TestTupleToV1Beta1Relationship_WithoutSubjectRelation(t *testing.T) {
	tuple := testRelationsTuple(t, "ns", "host", "r1", "viewer", "rbac", "workspace", "ws-1")

	rel, err := tupleToV1Beta1Relationship(tuple)
	require.NoError(t, err)
	require.NotNil(t, rel.Subject)
	assert.Equal(t, "", rel.Subject.GetRelation())

	filter, err := tupleToV1Beta1Filter(tuple)
	require.NoError(t, err)
	require.NotNil(t, filter.SubjectFilter)
	assert.Equal(t, "", filter.SubjectFilter.GetRelation())
}

func TestRelationsSubjectToSubjectReference_InvalidSubjectType(t *testing.T) {
	rid, err := model.NewLocalResourceId("x")
	require.NoError(t, err)
	subjectRes := model.NewRelationsResource(rid, model.NewRelationsObjectType("", "rbac"))
	rs := model.NewRelationsSubject(subjectRes, "")

	_, err = relationsSubjectToSubjectReference(rs)
	require.Error(t, err)
}

func testRelationsTuple(t *testing.T, resourceNamespace, resourceType, resourceID, relation, subjectNamespace, subjectType, subjectID string) model.RelationsTuple {
	t.Helper()
	rid, err := model.NewLocalResourceId(resourceID)
	require.NoError(t, err)
	resource := model.NewRelationsResource(rid, model.NewRelationsObjectType(resourceType, resourceNamespace))
	sid, err := model.NewLocalResourceId(subjectID)
	require.NoError(t, err)
	subject := model.NewRelationsSubject(model.NewRelationsResource(sid, model.NewRelationsObjectType(subjectType, subjectNamespace)), "")
	return model.NewRelationsTuple(resource, relation, subject)
}
