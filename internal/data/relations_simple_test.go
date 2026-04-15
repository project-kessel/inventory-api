package data

import (
	"context"
	"io"
	"sort"
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

func testPrincipalSubject(id string) *kessel.SubjectReference {
	return &kessel.SubjectReference{
		Subject: &kessel.ObjectReference{
			Type: &kessel.ObjectType{Namespace: "rbac", Name: "principal"},
			Id:   id,
		},
	}
}

func testCheckBulkItem(namespace, resourceType, resourceID, relation, subjectID string) *kessel.CheckBulkRequestItem {
	return &kessel.CheckBulkRequestItem{
		Resource: &kessel.ObjectReference{
			Type: &kessel.ObjectType{Namespace: namespace, Name: resourceType},
			Id:   resourceID,
		},
		Relation: relation,
		Subject:  testPrincipalSubject(subjectID),
	}
}

func TestSimpleRelationsRepository_RequiresExactMatch(t *testing.T) {
	repo := NewSimpleRelationsRepository()
	repo.Grant("user-123", "view", "hbi", "host", "resource-1")

	baseSub := testPrincipalSubject("user-123")
	wrongSub := testPrincipalSubject("other-user")

	tests := []struct {
		name            string
		namespace       string
		permission      string
		resourceType    string
		resourceID      string
		subjectOverride *kessel.SubjectReference
	}{
		{"wrong subject", "hbi", "view", "host", "resource-1", wrongSub},
		{"wrong permission", "hbi", "edit", "host", "resource-1", baseSub},
		{"wrong namespace", "other", "view", "host", "resource-1", baseSub},
		{"wrong resource type", "hbi", "view", "vm", "resource-1", baseSub},
		{"wrong resource id", "hbi", "view", "host", "resource-2", baseSub},
		{
			"wrong subject namespace", "hbi", "view", "host", "resource-1",
			&kessel.SubjectReference{
				Subject: &kessel.ObjectReference{
					Type: &kessel.ObjectType{Namespace: "other", Name: "principal"},
					Id:   "user-123",
				},
			},
		},
		{
			"wrong subject type", "hbi", "view", "host", "resource-1",
			&kessel.SubjectReference{
				Subject: &kessel.ObjectReference{
					Type: &kessel.ObjectType{Namespace: "rbac", Name: "group"},
					Id:   "user-123",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sub := tt.subjectOverride
			if sub == nil {
				sub = baseSub
			}
			allowed, _, err := repo.Check(context.Background(),
				tt.namespace, tt.permission, "", tt.resourceType, tt.resourceID, sub)
			require.NoError(t, err)
			assert.Equal(t, kessel.CheckResponse_ALLOWED_FALSE, allowed)
		})
	}
}

func TestSimpleRelationsRepository_Reset(t *testing.T) {
	repo := NewSimpleRelationsRepository()
	repo.Grant("user-123", "view", "hbi", "host", "resource-1")

	allowed, _, err := repo.Check(context.Background(),
		"hbi", "view", "", "host", "resource-1", testPrincipalSubject("user-123"))
	require.NoError(t, err)
	assert.Equal(t, kessel.CheckResponse_ALLOWED_TRUE, allowed)

	repo.Reset()

	allowed, _, err = repo.Check(context.Background(),
		"hbi", "view", "", "host", "resource-1", testPrincipalSubject("user-123"))
	require.NoError(t, err)
	assert.Equal(t, kessel.CheckResponse_ALLOWED_FALSE, allowed)
}

func TestSimpleRelationsRepository_CheckForUpdate(t *testing.T) {
	repo := NewSimpleRelationsRepository()
	repo.Grant("user-123", "edit", "hbi", "host", "resource-1")

	allowed, _, err := repo.CheckForUpdate(context.Background(),
		"hbi", "edit", "host", "resource-1", testPrincipalSubject("user-123"))
	require.NoError(t, err)
	assert.Equal(t, kessel.CheckForUpdateResponse_ALLOWED_TRUE, allowed)
}

func TestSimpleRelationsRepository_CheckBulk(t *testing.T) {
	repo := NewSimpleRelationsRepository()
	repo.Grant("user-123", "view", "hbi", "host", "resource-1")

	resp, err := repo.CheckBulk(context.Background(), &kessel.CheckBulkRequest{
		Items: []*kessel.CheckBulkRequestItem{
			testCheckBulkItem("hbi", "host", "resource-1", "view", "user-123"),
			testCheckBulkItem("hbi", "host", "resource-2", "view", "user-123"),
		},
	})
	require.NoError(t, err)
	require.Len(t, resp.Pairs, 2)
	assert.Equal(t, kessel.CheckBulkResponseItem_ALLOWED_TRUE, resp.Pairs[0].GetItem().GetAllowed())
	assert.Equal(t, kessel.CheckBulkResponseItem_ALLOWED_FALSE, resp.Pairs[1].GetItem().GetAllowed())
}

func TestSimpleRelationsRepository_CheckForUpdateBulk(t *testing.T) {
	repo := NewSimpleRelationsRepository()
	repo.Grant("user-123", "view", "hbi", "host", "resource-1")

	resp, err := repo.CheckForUpdateBulk(context.Background(), &kessel.CheckForUpdateBulkRequest{
		Items: []*kessel.CheckBulkRequestItem{
			testCheckBulkItem("hbi", "host", "resource-1", "view", "user-123"),
			testCheckBulkItem("hbi", "host", "resource-2", "view", "user-123"),
		},
	})
	require.NoError(t, err)
	require.Len(t, resp.Pairs, 2)
	assert.Equal(t, kessel.CheckBulkResponseItem_ALLOWED_TRUE, resp.Pairs[0].GetItem().GetAllowed())
	assert.Equal(t, kessel.CheckBulkResponseItem_ALLOWED_FALSE, resp.Pairs[1].GetItem().GetAllowed())
	require.NotNil(t, resp.ConsistencyToken)
	assert.NotEmpty(t, resp.ConsistencyToken.Token)
}

func TestSimpleRelationsRepository_MultipleTuples(t *testing.T) {
	repo := NewSimpleRelationsRepository()

	_, err := repo.CreateTuples(context.Background(), &kessel.CreateTuplesRequest{
		Tuples: []*kessel.Relationship{
			{
				Resource: &kessel.ObjectReference{
					Type: &kessel.ObjectType{Namespace: "hbi", Name: "host"},
					Id:   "resource-1",
				},
				Relation: "view",
				Subject:  testPrincipalSubject("user-a"),
			},
			{
				Resource: &kessel.ObjectReference{
					Type: &kessel.ObjectType{Namespace: "hbi", Name: "host"},
					Id:   "resource-2",
				},
				Relation: "view",
				Subject:  testPrincipalSubject("user-b"),
			},
		},
	})
	require.NoError(t, err)

	allowed, _, err := repo.Check(context.Background(),
		"hbi", "view", "", "host", "resource-1", testPrincipalSubject("user-a"))
	require.NoError(t, err)
	assert.Equal(t, kessel.CheckResponse_ALLOWED_TRUE, allowed)

	allowed, _, err = repo.Check(context.Background(),
		"hbi", "view", "", "host", "resource-2", testPrincipalSubject("user-b"))
	require.NoError(t, err)
	assert.Equal(t, kessel.CheckResponse_ALLOWED_TRUE, allowed)
}

func TestSimpleRelationsRepository_LookupResources(t *testing.T) {
	repo := NewSimpleRelationsRepository()
	repo.Grant("user-a", "view", "hbi", "host", "resource-1")
	repo.Grant("user-a", "view", "hbi", "host", "resource-2")
	repo.Grant("user-b", "view", "hbi", "host", "resource-3")

	stream, err := repo.LookupResources(context.Background(), &kessel.LookupResourcesRequest{
		ResourceType: &kessel.ObjectType{Namespace: "hbi", Name: "host"},
		Relation:     "view",
		Subject:      testPrincipalSubject("user-a"),
	})
	require.NoError(t, err)

	var ids []string
	for {
		msg, recvErr := stream.Recv()
		if recvErr == io.EOF {
			break
		}
		require.NoError(t, recvErr)
		ids = append(ids, msg.GetResource().GetId())
	}
	sort.Strings(ids)
	assert.Equal(t, []string{"resource-1", "resource-2"}, ids)
}

func TestSimpleRelationsRepository_LookupResources_Empty(t *testing.T) {
	repo := NewSimpleRelationsRepository()

	stream, err := repo.LookupResources(context.Background(), &kessel.LookupResourcesRequest{
		ResourceType: &kessel.ObjectType{Namespace: "hbi", Name: "host"},
		Relation:     "view",
		Subject:      testPrincipalSubject("user-a"),
	})
	require.NoError(t, err)

	_, err = stream.Recv()
	require.ErrorIs(t, err, io.EOF)
}

func TestSimpleRelationsRepository_LookupSubjects(t *testing.T) {
	repo := NewSimpleRelationsRepository()
	repo.Grant("user-a", "view", "hbi", "host", "resource-1")
	repo.Grant("user-b", "view", "hbi", "host", "resource-1")
	repo.Grant("user-c", "view", "hbi", "host", "resource-2")

	stream, err := repo.LookupSubjects(context.Background(), &kessel.LookupSubjectsRequest{
		Resource: &kessel.ObjectReference{
			Type: &kessel.ObjectType{Namespace: "hbi", Name: "host"},
			Id:   "resource-1",
		},
		Relation:    "view",
		SubjectType: &kessel.ObjectType{Namespace: "rbac", Name: "principal"},
	})
	require.NoError(t, err)

	var ids []string
	for {
		msg, recvErr := stream.Recv()
		if recvErr == io.EOF {
			break
		}
		require.NoError(t, recvErr)
		ids = append(ids, msg.GetSubject().GetSubject().GetId())
	}
	sort.Strings(ids)
	assert.Equal(t, []string{"user-a", "user-b"}, ids)
}

func TestSimpleRelationsRepository_Version_AdvancesOnMutations(t *testing.T) {
	repo := NewSimpleRelationsRepository()
	assert.Equal(t, int64(1), repo.Version())

	repo.Grant("user-a", "view", "hbi", "host", "resource-1")
	assert.Equal(t, int64(2), repo.Version())

	_, err := repo.CreateTuples(context.Background(), &kessel.CreateTuplesRequest{
		Tuples: []*kessel.Relationship{
			{
				Resource: &kessel.ObjectReference{
					Type: &kessel.ObjectType{Namespace: "hbi", Name: "host"},
					Id:   "resource-2",
				},
				Relation: "view",
				Subject:  testPrincipalSubject("user-b"),
			},
		},
	})
	require.NoError(t, err)
	assert.Equal(t, int64(3), repo.Version())

	ns := "hbi"
	rt := "host"
	rid := "resource-2"
	rel := "view"
	sns := "rbac"
	st := "principal"
	sid := "user-b"
	_, err = repo.DeleteTuples(context.Background(), &kessel.DeleteTuplesRequest{
		Filter: &kessel.RelationTupleFilter{
			ResourceNamespace: &ns,
			ResourceType:      &rt,
			ResourceId:        &rid,
			Relation:          &rel,
			SubjectFilter: &kessel.SubjectFilter{
				SubjectNamespace: &sns,
				SubjectType:      &st,
				SubjectId:        &sid,
			},
		},
	})
	require.NoError(t, err)
	assert.Equal(t, int64(4), repo.Version())
}

func TestSimpleRelationsRepository_Version_ResetRestoresInitialVersion(t *testing.T) {
	repo := NewSimpleRelationsRepository()
	repo.Grant("user-a", "view", "hbi", "host", "resource-1")
	repo.Grant("user-b", "view", "hbi", "host", "resource-2")
	assert.Equal(t, int64(3), repo.Version())

	repo.Reset()
	assert.Equal(t, int64(1), repo.Version())
}

func TestSimpleRelationsRepository_Snapshot_RetainAndCheck(t *testing.T) {
	repo := NewSimpleRelationsRepository()
	repo.Grant("user-a", "view", "hbi", "host", "resource-1")
	assert.Equal(t, int64(2), repo.Version())

	repo.RetainCurrentSnapshot()

	ns := "hbi"
	rt := "host"
	rid := "resource-1"
	rel := "view"
	sns := "rbac"
	st := "principal"
	sid := "user-a"
	_, err := repo.DeleteTuples(context.Background(), &kessel.DeleteTuplesRequest{
		Filter: &kessel.RelationTupleFilter{
			ResourceNamespace: &ns,
			ResourceType:      &rt,
			ResourceId:        &rid,
			Relation:          &rel,
			SubjectFilter: &kessel.SubjectFilter{
				SubjectNamespace: &sns,
				SubjectType:      &st,
				SubjectId:        &sid,
			},
		},
	})
	require.NoError(t, err)
	assert.Equal(t, int64(3), repo.Version())

	allowed, _, err := repo.Check(context.Background(),
		"hbi", "view", "", "host", "resource-1", testPrincipalSubject("user-a"))
	require.NoError(t, err)
	assert.Equal(t, kessel.CheckResponse_ALLOWED_TRUE, allowed)

	allowed, _, err = repo.Check(context.Background(),
		"hbi", "view", simpleFormatConsistencyToken(3), "host", "resource-1", testPrincipalSubject("user-a"))
	require.NoError(t, err)
	assert.Equal(t, kessel.CheckResponse_ALLOWED_FALSE, allowed)

	allowed, _, err = repo.Check(context.Background(),
		"hbi", "view", simpleFormatConsistencyToken(2), "host", "resource-1", testPrincipalSubject("user-a"))
	require.NoError(t, err)
	assert.Equal(t, kessel.CheckResponse_ALLOWED_TRUE, allowed)
}

func TestSimpleRelationsRepository_Snapshot_NoRetained_UsesLatest(t *testing.T) {
	repo := NewSimpleRelationsRepository()
	repo.Grant("user-a", "view", "hbi", "host", "resource-1")
	assert.Equal(t, int64(2), repo.Version())

	allowed, _, err := repo.Check(context.Background(),
		"hbi", "view", simpleFormatConsistencyToken(1), "host", "resource-1", testPrincipalSubject("user-a"))
	require.NoError(t, err)
	assert.Equal(t, kessel.CheckResponse_ALLOWED_TRUE, allowed)
}

func TestSimpleRelationsRepository_Snapshot_FindsOldestAtLeastAsFresh(t *testing.T) {
	repo := NewSimpleRelationsRepository()
	repo.Grant("user-a", "view", "hbi", "host", "resource-1")
	repo.RetainCurrentSnapshot()

	repo.Grant("user-a", "view", "hbi", "host", "resource-2")
	repo.RetainCurrentSnapshot()

	ns := "hbi"
	rt := "host"
	rid := "resource-1"
	rel := "view"
	sns := "rbac"
	st := "principal"
	sid := "user-a"
	_, err := repo.DeleteTuples(context.Background(), &kessel.DeleteTuplesRequest{
		Filter: &kessel.RelationTupleFilter{
			ResourceNamespace: &ns,
			ResourceType:      &rt,
			ResourceId:        &rid,
			Relation:          &rel,
			SubjectFilter: &kessel.SubjectFilter{
				SubjectNamespace: &sns,
				SubjectType:      &st,
				SubjectId:        &sid,
			},
		},
	})
	require.NoError(t, err)
	assert.Equal(t, int64(4), repo.Version())

	allowed, _, err := repo.Check(context.Background(),
		"hbi", "view", simpleFormatConsistencyToken(2), "host", "resource-1", testPrincipalSubject("user-a"))
	require.NoError(t, err)
	assert.Equal(t, kessel.CheckResponse_ALLOWED_TRUE, allowed)

	allowed, _, err = repo.Check(context.Background(),
		"hbi", "view", simpleFormatConsistencyToken(3), "host", "resource-1", testPrincipalSubject("user-a"))
	require.NoError(t, err)
	assert.Equal(t, kessel.CheckResponse_ALLOWED_TRUE, allowed)

	allowed, _, err = repo.Check(context.Background(),
		"hbi", "view", simpleFormatConsistencyToken(4), "host", "resource-1", testPrincipalSubject("user-a"))
	require.NoError(t, err)
	assert.Equal(t, kessel.CheckResponse_ALLOWED_FALSE, allowed)

	allowed, _, err = repo.Check(context.Background(),
		"hbi", "view", simpleFormatConsistencyToken(4), "host", "resource-2", testPrincipalSubject("user-a"))
	require.NoError(t, err)
	assert.Equal(t, kessel.CheckResponse_ALLOWED_TRUE, allowed)
}

func TestSimpleRelationsRepository_Snapshot_Release(t *testing.T) {
	repo := NewSimpleRelationsRepository()
	repo.Grant("user-a", "view", "hbi", "host", "resource-1")
	v := repo.RetainCurrentSnapshot()

	ns := "hbi"
	rt := "host"
	rid := "resource-1"
	rel := "view"
	sns := "rbac"
	st := "principal"
	sid := "user-a"
	_, err := repo.DeleteTuples(context.Background(), &kessel.DeleteTuplesRequest{
		Filter: &kessel.RelationTupleFilter{
			ResourceNamespace: &ns,
			ResourceType:      &rt,
			ResourceId:        &rid,
			Relation:          &rel,
			SubjectFilter: &kessel.SubjectFilter{
				SubjectNamespace: &sns,
				SubjectType:      &st,
				SubjectId:        &sid,
			},
		},
	})
	require.NoError(t, err)

	allowed, _, err := repo.Check(context.Background(),
		"hbi", "view", simpleFormatConsistencyToken(v), "host", "resource-1", testPrincipalSubject("user-a"))
	require.NoError(t, err)
	assert.Equal(t, kessel.CheckResponse_ALLOWED_TRUE, allowed)

	repo.ReleaseSnapshot(v)

	allowed, _, err = repo.Check(context.Background(),
		"hbi", "view", simpleFormatConsistencyToken(v), "host", "resource-1", testPrincipalSubject("user-a"))
	require.NoError(t, err)
	assert.Equal(t, kessel.CheckResponse_ALLOWED_FALSE, allowed)
}

func TestSimpleRelationsRepository_Snapshot_ClearAll(t *testing.T) {
	repo := NewSimpleRelationsRepository()
	repo.Grant("user-a", "view", "hbi", "host", "resource-1")
	repo.RetainCurrentSnapshot()
	repo.Grant("user-a", "view", "hbi", "host", "resource-2")
	repo.RetainCurrentSnapshot()

	repo.ClearSnapshots()

	allowed, _, err := repo.Check(context.Background(),
		"hbi", "view", simpleFormatConsistencyToken(2), "host", "resource-1", testPrincipalSubject("user-a"))
	require.NoError(t, err)
	assert.Equal(t, kessel.CheckResponse_ALLOWED_TRUE, allowed)
}

func TestSimpleRelationsRepository_CheckBulk_WithConsistencyToken(t *testing.T) {
	repo := NewSimpleRelationsRepository()
	repo.Grant("user-a", "view", "hbi", "host", "resource-1")
	repo.RetainCurrentSnapshot()

	ns := "hbi"
	rt := "host"
	rid := "resource-1"
	rel := "view"
	sns := "rbac"
	st := "principal"
	sid := "user-a"
	_, err := repo.DeleteTuples(context.Background(), &kessel.DeleteTuplesRequest{
		Filter: &kessel.RelationTupleFilter{
			ResourceNamespace: &ns,
			ResourceType:      &rt,
			ResourceId:        &rid,
			Relation:          &rel,
			SubjectFilter: &kessel.SubjectFilter{
				SubjectNamespace: &sns,
				SubjectType:      &st,
				SubjectId:        &sid,
			},
		},
	})
	require.NoError(t, err)

	item := testCheckBulkItem("hbi", "host", "resource-1", "view", "user-a")

	respOldest, err := repo.CheckBulk(context.Background(), &kessel.CheckBulkRequest{
		Items: []*kessel.CheckBulkRequestItem{item},
	})
	require.NoError(t, err)
	assert.Equal(t, kessel.CheckBulkResponseItem_ALLOWED_TRUE, respOldest.Pairs[0].GetItem().GetAllowed())

	respLatest, err := repo.CheckBulk(context.Background(), &kessel.CheckBulkRequest{
		Items: []*kessel.CheckBulkRequestItem{item},
		Consistency: &kessel.Consistency{
			Requirement: &kessel.Consistency_AtLeastAsFresh{
				AtLeastAsFresh: &kessel.ConsistencyToken{Token: simpleFormatConsistencyToken(repo.Version())},
			},
		},
	})
	require.NoError(t, err)
	assert.Equal(t, kessel.CheckBulkResponseItem_ALLOWED_FALSE, respLatest.Pairs[0].GetItem().GetAllowed())

	respSnap, err := repo.CheckBulk(context.Background(), &kessel.CheckBulkRequest{
		Items: []*kessel.CheckBulkRequestItem{item},
		Consistency: &kessel.Consistency{
			Requirement: &kessel.Consistency_AtLeastAsFresh{
				AtLeastAsFresh: &kessel.ConsistencyToken{Token: simpleFormatConsistencyToken(2)},
			},
		},
	})
	require.NoError(t, err)
	assert.Equal(t, kessel.CheckBulkResponseItem_ALLOWED_TRUE, respSnap.Pairs[0].GetItem().GetAllowed())
}
