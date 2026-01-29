package authz

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	kessel "github.com/project-kessel/relations-api/api/kessel/relations/v1beta1"
)

func TestSimpleAuthorizer_DefaultDeny(t *testing.T) {
	authz := NewSimpleAuthorizer()

	allowed, _, err := authz.Check(context.Background(),
		"hbi",        // namespace
		"view",       // permission
		"",           // consistencyToken
		"host",       // resourceType
		"resource-1", // localResourceID
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

func TestSimpleAuthorizer_CreateTuples_ThenCheck(t *testing.T) {
	authz := NewSimpleAuthorizer()

	// Create a tuple via the Authorizer interface
	_, err := authz.CreateTuples(context.Background(), &kessel.CreateTuplesRequest{
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

	// Check should now return allowed
	allowed, _, err := authz.Check(context.Background(),
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

func TestSimpleAuthorizer_DeleteTuples(t *testing.T) {
	authz := NewSimpleAuthorizer()

	// Create a tuple
	_, _ = authz.CreateTuples(context.Background(), &kessel.CreateTuplesRequest{
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

	// Delete the tuple using filter
	namespace := "hbi"
	resourceType := "host"
	resourceID := "resource-1"
	relation := "view"
	subjectNamespace := "rbac"
	subjectType := "principal"
	subjectID := "user-123"

	_, err := authz.DeleteTuples(context.Background(), &kessel.DeleteTuplesRequest{
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

	// Check should now return denied
	allowed, _, err := authz.Check(context.Background(),
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

func TestSimpleAuthorizer_Grant_ConvenienceMethod(t *testing.T) {
	authz := NewSimpleAuthorizer()

	// Grant is a convenience method that creates a tuple
	authz.Grant("user-123", "view", "hbi", "host", "resource-1")

	// Check should return allowed
	allowed, _, err := authz.Check(context.Background(),
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

func TestSimpleAuthorizer_RequiresExactMatch(t *testing.T) {
	authz := NewSimpleAuthorizer()
	authz.Grant("user-123", "view", "hbi", "host", "resource-1")

	tests := []struct {
		name             string
		subjectID        string
		permission       string
		namespace        string
		resourceType     string
		resourceID       string
		subjectNamespace string
		subjectType      string
	}{
		{"wrong subject", "user-999", "view", "hbi", "host", "resource-1", "rbac", "principal"},
		{"wrong permission", "user-123", "edit", "hbi", "host", "resource-1", "rbac", "principal"},
		{"wrong namespace", "user-123", "view", "acm", "host", "resource-1", "rbac", "principal"},
		{"wrong resourceType", "user-123", "view", "hbi", "cluster", "resource-1", "rbac", "principal"},
		{"wrong resourceID", "user-123", "view", "hbi", "host", "resource-999", "rbac", "principal"},
		{"wrong subject namespace", "user-123", "view", "hbi", "host", "resource-1", "other", "principal"},
		{"wrong subject type", "user-123", "view", "hbi", "host", "resource-1", "rbac", "group"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			allowed, _, err := authz.Check(context.Background(),
				tt.namespace,
				tt.permission,
				"",
				tt.resourceType,
				tt.resourceID,
				&kessel.SubjectReference{
					Subject: &kessel.ObjectReference{
						Type: &kessel.ObjectType{Namespace: tt.subjectNamespace, Name: tt.subjectType},
						Id:   tt.subjectID,
					},
				},
			)

			require.NoError(t, err)
			assert.Equal(t, kessel.CheckResponse_ALLOWED_FALSE, allowed,
				"expected deny for %s", tt.name)
		})
	}
}

func TestSimpleAuthorizer_Reset(t *testing.T) {
	authz := NewSimpleAuthorizer()
	authz.Grant("user-123", "view", "hbi", "host", "resource-1")

	// Verify tuple exists
	allowed, _, _ := authz.Check(context.Background(),
		"hbi", "view", "", "host", "resource-1",
		&kessel.SubjectReference{
			Subject: &kessel.ObjectReference{
				Type: &kessel.ObjectType{Namespace: "rbac", Name: "principal"},
				Id:   "user-123",
			},
		},
	)
	assert.Equal(t, kessel.CheckResponse_ALLOWED_TRUE, allowed)

	// Reset clears all tuples
	authz.Reset()

	allowed, _, _ = authz.Check(context.Background(),
		"hbi", "view", "", "host", "resource-1",
		&kessel.SubjectReference{
			Subject: &kessel.ObjectReference{
				Type: &kessel.ObjectType{Namespace: "rbac", Name: "principal"},
				Id:   "user-123",
			},
		},
	)
	assert.Equal(t, kessel.CheckResponse_ALLOWED_FALSE, allowed)
}

func TestSimpleAuthorizer_CheckForUpdate(t *testing.T) {
	authz := NewSimpleAuthorizer()
	authz.Grant("user-123", "edit", "hbi", "host", "resource-1")

	allowed, _, err := authz.CheckForUpdate(context.Background(),
		"hbi", "edit", "host", "resource-1",
		&kessel.SubjectReference{
			Subject: &kessel.ObjectReference{
				Type: &kessel.ObjectType{Namespace: "rbac", Name: "principal"},
				Id:   "user-123",
			},
		},
	)

	require.NoError(t, err)
	assert.Equal(t, kessel.CheckForUpdateResponse_ALLOWED_TRUE, allowed)
}

func TestSimpleAuthorizer_CheckBulk(t *testing.T) {
	authz := NewSimpleAuthorizer()
	authz.Grant("user-a", "view", "hbi", "host", "resource-1")
	// No grant for user-b on resource-2

	req := &kessel.CheckBulkRequest{
		Items: []*kessel.CheckBulkRequestItem{
			{
				Resource: &kessel.ObjectReference{
					Type: &kessel.ObjectType{Namespace: "hbi", Name: "host"},
					Id:   "resource-1",
				},
				Subject: &kessel.SubjectReference{
					Subject: &kessel.ObjectReference{
						Type: &kessel.ObjectType{Namespace: "rbac", Name: "principal"},
						Id:   "user-a",
					},
				},
				Relation: "view",
			},
			{
				Resource: &kessel.ObjectReference{
					Type: &kessel.ObjectType{Namespace: "hbi", Name: "host"},
					Id:   "resource-2",
				},
				Subject: &kessel.SubjectReference{
					Subject: &kessel.ObjectReference{
						Type: &kessel.ObjectType{Namespace: "rbac", Name: "principal"},
						Id:   "user-b",
					},
				},
				Relation: "edit",
			},
		},
	}

	resp, err := authz.CheckBulk(context.Background(), req)

	require.NoError(t, err)
	require.Len(t, resp.Pairs, 2)
	assert.Equal(t, kessel.CheckBulkResponseItem_ALLOWED_TRUE, resp.Pairs[0].GetItem().Allowed)
	assert.Equal(t, kessel.CheckBulkResponseItem_ALLOWED_FALSE, resp.Pairs[1].GetItem().Allowed)
}

func TestSimpleAuthorizer_MultipleTuples(t *testing.T) {
	authz := NewSimpleAuthorizer()

	// Create multiple tuples at once
	_, err := authz.CreateTuples(context.Background(), &kessel.CreateTuplesRequest{
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
						Id:   "user-a",
					},
				},
			},
			{
				Resource: &kessel.ObjectReference{
					Type: &kessel.ObjectType{Namespace: "hbi", Name: "host"},
					Id:   "resource-2",
				},
				Relation: "edit",
				Subject: &kessel.SubjectReference{
					Subject: &kessel.ObjectReference{
						Type: &kessel.ObjectType{Namespace: "rbac", Name: "principal"},
						Id:   "user-b",
					},
				},
			},
		},
	})
	require.NoError(t, err)

	// Both checks should pass
	allowed1, _, _ := authz.Check(context.Background(),
		"hbi", "view", "", "host", "resource-1",
		&kessel.SubjectReference{
			Subject: &kessel.ObjectReference{
				Type: &kessel.ObjectType{Namespace: "rbac", Name: "principal"},
				Id:   "user-a",
			},
		},
	)
	assert.Equal(t, kessel.CheckResponse_ALLOWED_TRUE, allowed1)

	allowed2, _, _ := authz.Check(context.Background(),
		"hbi", "edit", "", "host", "resource-2",
		&kessel.SubjectReference{
			Subject: &kessel.ObjectReference{
				Type: &kessel.ObjectType{Namespace: "rbac", Name: "principal"},
				Id:   "user-b",
			},
		},
	)
	assert.Equal(t, kessel.CheckResponse_ALLOWED_TRUE, allowed2)
}

func TestSimpleAuthorizer_Health(t *testing.T) {
	authz := NewSimpleAuthorizer()

	resp, err := authz.Health(context.Background())

	require.NoError(t, err)
	assert.NotNil(t, resp)
}

func TestSimpleAuthorizer_LookupResources(t *testing.T) {
	authz := NewSimpleAuthorizer()

	// Create tuples for user-a
	_, _ = authz.CreateTuples(context.Background(), &kessel.CreateTuplesRequest{
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
						Id:   "user-a",
					},
				},
			},
			{
				Resource: &kessel.ObjectReference{
					Type: &kessel.ObjectType{Namespace: "hbi", Name: "host"},
					Id:   "resource-2",
				},
				Relation: "view",
				Subject: &kessel.SubjectReference{
					Subject: &kessel.ObjectReference{
						Type: &kessel.ObjectType{Namespace: "rbac", Name: "principal"},
						Id:   "user-a",
					},
				},
			},
			// This one is for a different user
			{
				Resource: &kessel.ObjectReference{
					Type: &kessel.ObjectType{Namespace: "hbi", Name: "host"},
					Id:   "resource-3",
				},
				Relation: "view",
				Subject: &kessel.SubjectReference{
					Subject: &kessel.ObjectReference{
						Type: &kessel.ObjectType{Namespace: "rbac", Name: "principal"},
						Id:   "user-b",
					},
				},
			},
		},
	})

	// Lookup resources for user-a with "view" relation
	stream, err := authz.LookupResources(context.Background(), &kessel.LookupResourcesRequest{
		ResourceType: &kessel.ObjectType{Namespace: "hbi", Name: "host"},
		Relation:     "view",
		Subject: &kessel.SubjectReference{
			Subject: &kessel.ObjectReference{
				Type: &kessel.ObjectType{Namespace: "rbac", Name: "principal"},
				Id:   "user-a",
			},
		},
	})
	require.NoError(t, err)

	// Collect all results
	var resourceIDs []string
	for {
		resp, err := stream.Recv()
		if err != nil {
			break
		}
		resourceIDs = append(resourceIDs, resp.Resource.Id)
	}

	// Should find 2 resources for user-a (not user-b's resource)
	assert.Len(t, resourceIDs, 2)
	assert.Contains(t, resourceIDs, "resource-1")
	assert.Contains(t, resourceIDs, "resource-2")
	assert.NotContains(t, resourceIDs, "resource-3")
}

func TestSimpleAuthorizer_LookupResources_Empty(t *testing.T) {
	authz := NewSimpleAuthorizer()

	// Lookup with no tuples
	stream, err := authz.LookupResources(context.Background(), &kessel.LookupResourcesRequest{
		ResourceType: &kessel.ObjectType{Namespace: "hbi", Name: "host"},
		Relation:     "view",
		Subject: &kessel.SubjectReference{
			Subject: &kessel.ObjectReference{
				Type: &kessel.ObjectType{Namespace: "rbac", Name: "principal"},
				Id:   "user-a",
			},
		},
	})
	require.NoError(t, err)

	// Should return empty (EOF immediately)
	resp, err := stream.Recv()
	assert.Nil(t, resp)
	assert.Error(t, err) // io.EOF
}

// Tests for versioning and snapshot support

func TestSimpleAuthorizer_Version_AdvancesOnMutations(t *testing.T) {
	authz := NewSimpleAuthorizer()

	// Initial version is 1
	assert.Equal(t, int64(1), authz.Version())

	// Grant advances version
	authz.Grant("user-a", "view", "hbi", "host", "resource-1")
	assert.Equal(t, int64(2), authz.Version())

	// CreateTuples advances version
	_, _ = authz.CreateTuples(context.Background(), &kessel.CreateTuplesRequest{
		Tuples: []*kessel.Relationship{
			{
				Resource: &kessel.ObjectReference{
					Type: &kessel.ObjectType{Namespace: "hbi", Name: "host"},
					Id:   "resource-2",
				},
				Relation: "view",
				Subject: &kessel.SubjectReference{
					Subject: &kessel.ObjectReference{
						Type: &kessel.ObjectType{Namespace: "rbac", Name: "principal"},
						Id:   "user-b",
					},
				},
			},
		},
	})
	assert.Equal(t, int64(3), authz.Version())

	// DeleteTuples advances version (even with no matches)
	_, _ = authz.DeleteTuples(context.Background(), &kessel.DeleteTuplesRequest{})
	assert.Equal(t, int64(4), authz.Version())
}

func TestSimpleAuthorizer_Version_ResetRestoresInitialVersion(t *testing.T) {
	authz := NewSimpleAuthorizer()
	authz.Grant("user-a", "view", "hbi", "host", "resource-1")
	authz.Grant("user-b", "view", "hbi", "host", "resource-2")
	assert.Equal(t, int64(3), authz.Version())

	authz.Reset()
	assert.Equal(t, int64(1), authz.Version())
}

func TestSimpleAuthorizer_ConsistencyToken_ReturnsVersion(t *testing.T) {
	authz := NewSimpleAuthorizer()

	// Check returns consistency token
	_, token, err := authz.Check(context.Background(),
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

	// After mutation, token reflects new version
	authz.Grant("user-a", "view", "hbi", "host", "resource-1")

	_, token, err = authz.Check(context.Background(),
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

func TestSimpleAuthorizer_Snapshot_RetainAndCheck(t *testing.T) {
	authz := NewSimpleAuthorizer()

	// Grant tuple at version 1 -> becomes version 2
	authz.Grant("user-a", "view", "hbi", "host", "resource-1")
	assert.Equal(t, int64(2), authz.Version())

	// Retain snapshot at version 2
	snapshotVersion := authz.RetainCurrentSnapshot()
	assert.Equal(t, int64(2), snapshotVersion)

	// Make changes: remove the tuple -> version 3
	namespace := "hbi"
	resourceType := "host"
	resourceID := "resource-1"
	relation := "view"
	subjectNamespace := "rbac"
	subjectType := "principal"
	subjectID := "user-a"

	_, _ = authz.DeleteTuples(context.Background(), &kessel.DeleteTuplesRequest{
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
	assert.Equal(t, int64(3), authz.Version())

	// Check with no token (oldest available) -> uses retained snapshot v2 -> allowed
	// The oldest available snapshot >= 0 is v2 (retained), not v3 (current)
	allowed, _, _ := authz.Check(context.Background(),
		"hbi", "view", "", "host", "resource-1",
		&kessel.SubjectReference{
			Subject: &kessel.ObjectReference{
				Type: &kessel.ObjectType{Namespace: "rbac", Name: "principal"},
				Id:   "user-a",
			},
		},
	)
	assert.Equal(t, kessel.CheckResponse_ALLOWED_TRUE, allowed, "no token uses oldest available (v2 snapshot, allowed)")

	// Check with token >= current version -> uses current (v3) -> denied
	allowed, _, _ = authz.Check(context.Background(),
		"hbi", "view", "3", "host", "resource-1",
		&kessel.SubjectReference{
			Subject: &kessel.ObjectReference{
				Type: &kessel.ObjectType{Namespace: "rbac", Name: "principal"},
				Id:   "user-a",
			},
		},
	)
	assert.Equal(t, kessel.CheckResponse_ALLOWED_FALSE, allowed, "token 3 uses current (v3), denied")

	// Check with old consistency token -> uses retained snapshot v2 -> allowed
	allowed, _, _ = authz.Check(context.Background(),
		"hbi", "view", "2", "host", "resource-1",
		&kessel.SubjectReference{
			Subject: &kessel.ObjectReference{
				Type: &kessel.ObjectType{Namespace: "rbac", Name: "principal"},
				Id:   "user-a",
			},
		},
	)
	assert.Equal(t, kessel.CheckResponse_ALLOWED_TRUE, allowed, "token 2 uses snapshot v2, allowed")
}

func TestSimpleAuthorizer_Snapshot_NoRetained_UsesLatest(t *testing.T) {
	authz := NewSimpleAuthorizer()

	authz.Grant("user-a", "view", "hbi", "host", "resource-1")

	// Without retaining, old consistency tokens use latest state
	// Grant another tuple to advance version
	authz.Grant("user-b", "view", "hbi", "host", "resource-2")

	// Now delete user-a's tuple
	namespace := "hbi"
	resourceType := "host"
	resourceID := "resource-1"
	relation := "view"
	subjectNamespace := "rbac"
	subjectType := "principal"
	subjectID := "user-a"

	_, _ = authz.DeleteTuples(context.Background(), &kessel.DeleteTuplesRequest{
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

	// Check with old token "2" (but no snapshot retained) -> uses latest -> denied
	allowed, _, _ := authz.Check(context.Background(),
		"hbi", "view", "2", "host", "resource-1",
		&kessel.SubjectReference{
			Subject: &kessel.ObjectReference{
				Type: &kessel.ObjectType{Namespace: "rbac", Name: "principal"},
				Id:   "user-a",
			},
		},
	)
	assert.Equal(t, kessel.CheckResponse_ALLOWED_FALSE, allowed,
		"without retained snapshot, should use latest state")
}

func TestSimpleAuthorizer_Snapshot_FindsOldestAtLeastAsFresh(t *testing.T) {
	authz := NewSimpleAuthorizer()

	// Setup: v1 -> grant A -> v2
	authz.Grant("user-a", "view", "hbi", "host", "resource-A")
	assert.Equal(t, int64(2), authz.Version())
	authz.RetainCurrentSnapshot() // Retain v2

	// v2 -> grant B -> v3
	authz.Grant("user-b", "view", "hbi", "host", "resource-B")
	assert.Equal(t, int64(3), authz.Version())
	authz.RetainCurrentSnapshot() // Retain v3

	// v3 -> grant C -> v4
	authz.Grant("user-c", "view", "hbi", "host", "resource-C")
	assert.Equal(t, int64(4), authz.Version())

	// v4 -> remove A -> v5
	namespace := "hbi"
	resourceType := "host"
	resourceID := "resource-A"
	relation := "view"
	subjectNamespace := "rbac"
	subjectType := "principal"
	subjectID := "user-a"

	_, _ = authz.DeleteTuples(context.Background(), &kessel.DeleteTuplesRequest{
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
	assert.Equal(t, int64(5), authz.Version())

	// Token "2" should use snapshot v2 (oldest >= 2), which has A
	allowedA, _, _ := authz.Check(context.Background(),
		"hbi", "view", "2", "host", "resource-A",
		&kessel.SubjectReference{
			Subject: &kessel.ObjectReference{
				Type: &kessel.ObjectType{Namespace: "rbac", Name: "principal"},
				Id:   "user-a",
			},
		},
	)
	assert.Equal(t, kessel.CheckResponse_ALLOWED_TRUE, allowedA, "token 2 should use v2 snapshot with A")

	// Token "3" should use snapshot v3 (oldest >= 3), which has A and B
	allowedB, _, _ := authz.Check(context.Background(),
		"hbi", "view", "3", "host", "resource-B",
		&kessel.SubjectReference{
			Subject: &kessel.ObjectReference{
				Type: &kessel.ObjectType{Namespace: "rbac", Name: "principal"},
				Id:   "user-b",
			},
		},
	)
	assert.Equal(t, kessel.CheckResponse_ALLOWED_TRUE, allowedB, "token 3 should use v3 snapshot with B")

	// Token "4" or "5" should use latest (no retained snapshot >= 4)
	allowedC, _, _ := authz.Check(context.Background(),
		"hbi", "view", "4", "host", "resource-C",
		&kessel.SubjectReference{
			Subject: &kessel.ObjectReference{
				Type: &kessel.ObjectType{Namespace: "rbac", Name: "principal"},
				Id:   "user-c",
			},
		},
	)
	assert.Equal(t, kessel.CheckResponse_ALLOWED_TRUE, allowedC, "token 4 should find C in latest")

	// A is deleted in latest
	allowedALatest, _, _ := authz.Check(context.Background(),
		"hbi", "view", "5", "host", "resource-A",
		&kessel.SubjectReference{
			Subject: &kessel.ObjectReference{
				Type: &kessel.ObjectType{Namespace: "rbac", Name: "principal"},
				Id:   "user-a",
			},
		},
	)
	assert.Equal(t, kessel.CheckResponse_ALLOWED_FALSE, allowedALatest, "A deleted at v5")
}

func TestSimpleAuthorizer_Snapshot_Release(t *testing.T) {
	authz := NewSimpleAuthorizer()

	authz.Grant("user-a", "view", "hbi", "host", "resource-1")
	v := authz.RetainCurrentSnapshot()

	// Mutate
	namespace := "hbi"
	resourceType := "host"
	resourceID := "resource-1"
	relation := "view"
	subjectNamespace := "rbac"
	subjectType := "principal"
	subjectID := "user-a"

	_, _ = authz.DeleteTuples(context.Background(), &kessel.DeleteTuplesRequest{
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

	// Check with snapshot token -> allowed
	allowed, _, _ := authz.Check(context.Background(),
		"hbi", "view", formatConsistencyToken(v), "host", "resource-1",
		&kessel.SubjectReference{
			Subject: &kessel.ObjectReference{
				Type: &kessel.ObjectType{Namespace: "rbac", Name: "principal"},
				Id:   "user-a",
			},
		},
	)
	assert.Equal(t, kessel.CheckResponse_ALLOWED_TRUE, allowed)

	// Release snapshot
	authz.ReleaseSnapshot(v)

	// Now same check falls back to latest -> denied
	allowed, _, _ = authz.Check(context.Background(),
		"hbi", "view", formatConsistencyToken(v), "host", "resource-1",
		&kessel.SubjectReference{
			Subject: &kessel.ObjectReference{
				Type: &kessel.ObjectType{Namespace: "rbac", Name: "principal"},
				Id:   "user-a",
			},
		},
	)
	assert.Equal(t, kessel.CheckResponse_ALLOWED_FALSE, allowed, "after release, falls back to latest")
}

func TestSimpleAuthorizer_Snapshot_ClearAll(t *testing.T) {
	authz := NewSimpleAuthorizer()

	authz.Grant("user-a", "view", "hbi", "host", "resource-1")
	authz.RetainCurrentSnapshot()
	authz.Grant("user-b", "view", "hbi", "host", "resource-2")
	authz.RetainCurrentSnapshot()

	// Clear all snapshots
	authz.ClearSnapshots()

	// Now only latest is available
	assert.Equal(t, int64(3), authz.Version())

	// Old tokens fall back to latest
	allowed, _, _ := authz.Check(context.Background(),
		"hbi", "view", "2", "host", "resource-1",
		&kessel.SubjectReference{
			Subject: &kessel.ObjectReference{
				Type: &kessel.ObjectType{Namespace: "rbac", Name: "principal"},
				Id:   "user-a",
			},
		},
	)
	assert.Equal(t, kessel.CheckResponse_ALLOWED_TRUE, allowed, "still allowed in latest")
}

func TestSimpleAuthorizer_CheckBulk_WithConsistencyToken(t *testing.T) {
	authz := NewSimpleAuthorizer()

	// Grant and retain snapshot at v2
	authz.Grant("user-a", "view", "hbi", "host", "resource-1")
	snapshotVersion := authz.RetainCurrentSnapshot()

	// Delete the tuple -> v3
	namespace := "hbi"
	resourceType := "host"
	resourceID := "resource-1"
	relation := "view"
	subjectNamespace := "rbac"
	subjectType := "principal"
	subjectID := "user-a"

	_, _ = authz.DeleteTuples(context.Background(), &kessel.DeleteTuplesRequest{
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
	currentVersion := authz.Version()

	// CheckBulk with no consistency token -> uses oldest available (v2) -> allowed
	resp, err := authz.CheckBulk(context.Background(), &kessel.CheckBulkRequest{
		Items: []*kessel.CheckBulkRequestItem{
			{
				Resource: &kessel.ObjectReference{
					Type: &kessel.ObjectType{Namespace: "hbi", Name: "host"},
					Id:   "resource-1",
				},
				Subject: &kessel.SubjectReference{
					Subject: &kessel.ObjectReference{
						Type: &kessel.ObjectType{Namespace: "rbac", Name: "principal"},
						Id:   "user-a",
					},
				},
				Relation: "view",
			},
		},
	})
	require.NoError(t, err)
	assert.Equal(t, kessel.CheckBulkResponseItem_ALLOWED_TRUE, resp.Pairs[0].GetItem().Allowed,
		"no token uses oldest available (v2 snapshot), allowed")

	// CheckBulk with token >= current version -> uses current (v3) -> denied
	resp, err = authz.CheckBulk(context.Background(), &kessel.CheckBulkRequest{
		Consistency: &kessel.Consistency{
			Requirement: &kessel.Consistency_AtLeastAsFresh{
				AtLeastAsFresh: &kessel.ConsistencyToken{
					Token: formatConsistencyToken(currentVersion),
				},
			},
		},
		Items: []*kessel.CheckBulkRequestItem{
			{
				Resource: &kessel.ObjectReference{
					Type: &kessel.ObjectType{Namespace: "hbi", Name: "host"},
					Id:   "resource-1",
				},
				Subject: &kessel.SubjectReference{
					Subject: &kessel.ObjectReference{
						Type: &kessel.ObjectType{Namespace: "rbac", Name: "principal"},
						Id:   "user-a",
					},
				},
				Relation: "view",
			},
		},
	})
	require.NoError(t, err)
	assert.Equal(t, kessel.CheckBulkResponseItem_ALLOWED_FALSE, resp.Pairs[0].GetItem().Allowed,
		"token at current version uses latest (v3), denied")

	// CheckBulk with old consistency token (v2) -> uses retained snapshot -> allowed
	resp, err = authz.CheckBulk(context.Background(), &kessel.CheckBulkRequest{
		Consistency: &kessel.Consistency{
			Requirement: &kessel.Consistency_AtLeastAsFresh{
				AtLeastAsFresh: &kessel.ConsistencyToken{
					Token: formatConsistencyToken(snapshotVersion),
				},
			},
		},
		Items: []*kessel.CheckBulkRequestItem{
			{
				Resource: &kessel.ObjectReference{
					Type: &kessel.ObjectType{Namespace: "hbi", Name: "host"},
					Id:   "resource-1",
				},
				Subject: &kessel.SubjectReference{
					Subject: &kessel.ObjectReference{
						Type: &kessel.ObjectType{Namespace: "rbac", Name: "principal"},
						Id:   "user-a",
					},
				},
				Relation: "view",
			},
		},
	})
	require.NoError(t, err)
	assert.Equal(t, kessel.CheckBulkResponseItem_ALLOWED_TRUE, resp.Pairs[0].GetItem().Allowed,
		"token v2 uses retained snapshot, allowed")
}
