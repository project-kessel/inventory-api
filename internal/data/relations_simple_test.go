package data

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	kessel "github.com/project-kessel/relations-api/api/kessel/relations/v1beta1"
)

func TestSimpleRelationsRepository_DefaultDeny(t *testing.T) {
	repo := NewSimpleRelationsRepository()

	allowed, _, err := repo.Check(context.Background(),
		"hbi", "view", "", "host", "resource-1",
		&kessel.SubjectReference{
			Subject: &kessel.ObjectReference{
				Type: &kessel.ObjectType{Namespace: "rbac", Name: "principal"},
				Id:   "user-123",
			},
		},
	)

	require.NoError(t, err)
	assert.Equal(t, kessel.CheckResponse_ALLOWED_FALSE, allowed)
}

func TestSimpleRelationsRepository_CreateTuples_ThenCheck(t *testing.T) {
	repo := NewSimpleRelationsRepository()

	_, err := repo.CreateTuples(context.Background(), &kessel.CreateTuplesRequest{
		Tuples: []*kessel.Relationship{
			{
				Resource: &kessel.ObjectReference{
					Type: &kessel.ObjectType{Namespace: "hbi", Name: "host"},
					Id:   "resource-1",
				},
				Relation: "view",
				Subject: &kessel.SubjectReference{
					Subject: &kessel.ObjectReference{
						Type: &kessel.ObjectType{Namespace: "rbac", Name: "principal"},
						Id:   "user-123",
					},
				},
			},
		},
	})
	require.NoError(t, err)

	allowed, _, err := repo.Check(context.Background(),
		"hbi", "view", "", "host", "resource-1",
		&kessel.SubjectReference{
			Subject: &kessel.ObjectReference{
				Type: &kessel.ObjectType{Namespace: "rbac", Name: "principal"},
				Id:   "user-123",
			},
		},
	)

	require.NoError(t, err)
	assert.Equal(t, kessel.CheckResponse_ALLOWED_TRUE, allowed)
}

func TestSimpleRelationsRepository_DeleteTuples(t *testing.T) {
	repo := NewSimpleRelationsRepository()

	_, _ = repo.CreateTuples(context.Background(), &kessel.CreateTuplesRequest{
		Tuples: []*kessel.Relationship{
			{
				Resource: &kessel.ObjectReference{
					Type: &kessel.ObjectType{Namespace: "hbi", Name: "host"},
					Id:   "resource-1",
				},
				Relation: "view",
				Subject: &kessel.SubjectReference{
					Subject: &kessel.ObjectReference{
						Type: &kessel.ObjectType{Namespace: "rbac", Name: "principal"},
						Id:   "user-123",
					},
				},
			},
		},
	})

	namespace := "hbi"
	resourceType := "host"
	resourceID := "resource-1"
	relation := "view"
	subjectNamespace := "rbac"
	subjectType := "principal"
	subjectID := "user-123"

	_, err := repo.DeleteTuples(context.Background(), &kessel.DeleteTuplesRequest{
		Filter: &kessel.RelationTupleFilter{
			ResourceNamespace: &namespace,
			ResourceType:      &resourceType,
			ResourceId:        &resourceID,
			Relation:          &relation,
			SubjectFilter: &kessel.SubjectFilter{
				SubjectNamespace: &subjectNamespace,
				SubjectType:      &subjectType,
				SubjectId:        &subjectID,
			},
		},
	})
	require.NoError(t, err)

	allowed, _, err := repo.Check(context.Background(),
		"hbi", "view", "", "host", "resource-1",
		&kessel.SubjectReference{
			Subject: &kessel.ObjectReference{
				Type: &kessel.ObjectType{Namespace: "rbac", Name: "principal"},
				Id:   "user-123",
			},
		},
	)

	require.NoError(t, err)
	assert.Equal(t, kessel.CheckResponse_ALLOWED_FALSE, allowed)
}

func TestSimpleRelationsRepository_Grant(t *testing.T) {
	repo := NewSimpleRelationsRepository()
	repo.Grant("user-123", "view", "hbi", "host", "resource-1")

	allowed, _, err := repo.Check(context.Background(),
		"hbi", "view", "", "host", "resource-1",
		&kessel.SubjectReference{
			Subject: &kessel.ObjectReference{
				Type: &kessel.ObjectType{Namespace: "rbac", Name: "principal"},
				Id:   "user-123",
			},
		},
	)

	require.NoError(t, err)
	assert.Equal(t, kessel.CheckResponse_ALLOWED_TRUE, allowed)
}

func TestSimpleRelationsRepository_Health(t *testing.T) {
	repo := NewSimpleRelationsRepository()

	resp, err := repo.Health(context.Background())

	require.NoError(t, err)
	assert.NotNil(t, resp)
}

func TestSimpleRelationsRepository_Version(t *testing.T) {
	repo := NewSimpleRelationsRepository()
	assert.Equal(t, int64(1), repo.Version())

	repo.Grant("user-a", "view", "hbi", "host", "resource-1")
	assert.Equal(t, int64(2), repo.Version())
}

func TestSimpleRelationsRepository_ConsistencyToken(t *testing.T) {
	repo := NewSimpleRelationsRepository()

	_, token, err := repo.Check(context.Background(),
		"hbi", "view", "", "host", "resource-1",
		&kessel.SubjectReference{
			Subject: &kessel.ObjectReference{
				Type: &kessel.ObjectType{Namespace: "rbac", Name: "principal"},
				Id:   "user-a",
			},
		},
	)
	require.NoError(t, err)
	assert.Equal(t, "1", token.Token)

	repo.Grant("user-a", "view", "hbi", "host", "resource-1")

	_, token, err = repo.Check(context.Background(),
		"hbi", "view", "", "host", "resource-1",
		&kessel.SubjectReference{
			Subject: &kessel.ObjectReference{
				Type: &kessel.ObjectType{Namespace: "rbac", Name: "principal"},
				Id:   "user-a",
			},
		},
	)
	require.NoError(t, err)
	assert.Equal(t, "2", token.Token)
}
