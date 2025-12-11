package spicedb

import (
	"context"
	"fmt"
	"os"
	"testing"

	v1 "github.com/authzed/authzed-go/proto/authzed/api/v1"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/middleware/tracing"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/project-kessel/inventory-api/internal/authz/api"
	kessel "github.com/project-kessel/inventory-api/internal/authz/model"
)

var container *LocalSpiceDbContainer

// TestMain sets up a shared SpiceDB container for all tests in this package
func TestMain(m *testing.M) {
	var err error
	logger := log.With(log.NewStdLogger(os.Stdout),
		"ts", log.DefaultTimestamp,
		"caller", log.DefaultCaller,
		"trace.id", tracing.TraceID(),
		"span.id", tracing.SpanID(),
	)

	container, err = CreateContainer(&ContainerOptions{Logger: logger})

	if err != nil {
		fmt.Printf("Error initializing Docker container: %s\n", err)
		os.Exit(-1)
	}

	result := m.Run()

	container.Close()
	os.Exit(result)
}

// ============================================================================
// Basic Relationship Creation Tests
// ============================================================================

// TestCreateRelationship tests basic relationship creation
func TestCreateRelationship(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	spiceDbRepo, err := container.CreateSpiceDbRepository()
	assert.NoError(t, err)

	preExisting := CheckForRelationship(spiceDbRepo, "bob", "rbac", "principal", "",
		"member", "rbac", "group", "bob_club", nil)
	assert.False(t, preExisting)

	rels := []*kessel.Relationship{
		createRelationship("rbac", "group", "bob_club", "member", "rbac", "principal", "bob", ""),
	}

	touch := api.TouchSemantics(false)
	_, err = spiceDbRepo.CreateRelationships(ctx, rels, touch, nil)
	assert.NoError(t, err)

	container.WaitForQuantizationInterval()

	exists := CheckForRelationship(spiceDbRepo, "bob", "rbac", "principal", "",
		"member", "rbac", "group", "bob_club", nil)
	assert.True(t, exists)
}

// TestCreateRelationshipWithSubjectRelation tests relationship creation with subject relations
func TestCreateRelationshipWithSubjectRelation(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	spiceDbRepo, err := container.CreateSpiceDbRepository()
	assert.NoError(t, err)

	preExisting := CheckForRelationship(spiceDbRepo, "bob", "rbac", "principal", "", "member", "rbac", "group", "bob_club", nil)
	assert.False(t, preExisting)

	rels := []*kessel.Relationship{
		createRelationship("rbac", "group", "bob_club", "member", "rbac", "principal", "bob", ""),
		createRelationship("rbac", "role_binding", "fan_binding", "granted", "rbac", "role", "fan", ""),
		createRelationship("rbac", "role_binding", "fan_binding", "subject", "rbac", "group", "bob_club", "member"),
		createRelationship("rbac", "role", "fan", "view_widget", "rbac", "principal", "*", ""),
	}

	touch := api.TouchSemantics(false)

	_, err = spiceDbRepo.CreateRelationships(ctx, rels, touch, nil)
	assert.NoError(t, err)

	container.WaitForQuantizationInterval()

	exists := CheckForRelationship(spiceDbRepo, "bob", "rbac", "principal", "", "member", "rbac", "group", "bob_club", nil)
	assert.True(t, exists)

	exists = CheckForRelationship(spiceDbRepo, "bob_club", "rbac", "group", "member", "subject", "rbac", "role_binding", "fan_binding", nil)
	assert.True(t, exists)

	runSpiceDBCheck(t, ctx, spiceDbRepo, "principal", "rbac", "bob", "subject", "role_binding", "rbac", "fan_binding", kessel.AllowedTrue, nil)

	runSpiceDBCheck(t, ctx, spiceDbRepo, "principal", "rbac", "alice", "subject", "role_binding", "rbac", "fan_binding", kessel.AllowedFalse, nil)

	runSpiceDBCheck(t, ctx, spiceDbRepo, "principal", "rbac", "bob", "view_widget", "role_binding", "rbac", "fan_binding", kessel.AllowedTrue, nil)

	runSpiceDBCheck(t, ctx, spiceDbRepo, "principal", "rbac", "alice", "view_widget", "role_binding", "rbac", "fan_binding", kessel.AllowedFalse, nil)

	runSpiceDBCheck(t, ctx, spiceDbRepo, "role", "rbac", "fan", "granted", "role_binding", "rbac", "fan_binding", kessel.AllowedTrue, nil)

	runSpiceDBCheck(t, ctx, spiceDbRepo, "role", "rbac", "fake_fan", "granted", "role_binding", "rbac", "fan_binding", kessel.AllowedFalse, nil)
}

// TestCreateRelationshipWithConsistencyToken tests that consistency tokens work for read-after-write
func TestCreateRelationshipWithConsistencyToken(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	spiceDbRepo, err := container.CreateSpiceDbRepository()
	assert.NoError(t, err)

	preExisting := CheckForRelationship(spiceDbRepo, "bob", "rbac", "principal", "", "member", "rbac", "group", "bob_club", nil)
	assert.False(t, preExisting)

	rels := []*kessel.Relationship{
		createRelationship("rbac", "group", "bob_club", "member", "rbac", "principal", "bob", ""),
	}

	touch := api.TouchSemantics(false)

	resp, err := spiceDbRepo.CreateRelationships(ctx, rels, touch, nil)
	assert.NoError(t, err)

	exists := CheckForRelationship(spiceDbRepo, "bob", "rbac", "principal", "", "member", "rbac", "group", "bob_club",
		&kessel.Consistency{
			AtLeastAsFresh: resp.ConsistencyToken,
		},
	)
	assert.True(t, exists)
}

// ============================================================================
// Touch Semantics Tests
// ============================================================================

// TestSecondCreateRelationshipFailsWithTouchFalse verifies that duplicate creation fails when touch=false
func TestSecondCreateRelationshipFailsWithTouchFalse(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	spiceDbRepo, err := container.CreateSpiceDbRepository()
	assert.NoError(t, err)

	preExisting := CheckForRelationship(spiceDbRepo, "bob", "rbac", "principal", "", "member", "rbac", "group", "bob_club", nil)
	assert.False(t, preExisting)

	rels := []*kessel.Relationship{
		createRelationship("rbac", "group", "bob_club", "member", "rbac", "principal", "bob", ""),
	}

	touch := api.TouchSemantics(false)

	_, err = spiceDbRepo.CreateRelationships(ctx, rels, touch, nil)
	assert.NoError(t, err)

	_, err = spiceDbRepo.CreateRelationships(ctx, rels, touch, nil)
	assert.Error(t, err)

	container.WaitForQuantizationInterval()

	exists := CheckForRelationship(spiceDbRepo, "bob", "rbac", "principal", "", "member", "rbac", "group", "bob_club", nil)
	assert.True(t, exists)
}

// TestSecondCreateRelationshipSucceedsWithTouchTrue verifies that duplicate creation succeeds when touch=true
func TestSecondCreateRelationshipSucceedsWithTouchTrue(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	spiceDbRepo, err := container.CreateSpiceDbRepository()
	assert.NoError(t, err)

	preExisting := CheckForRelationship(spiceDbRepo, "bob", "rbac", "principal", "", "member", "rbac", "group", "bob_club", nil)
	assert.False(t, preExisting)

	rels := []*kessel.Relationship{
		createRelationship("rbac", "group", "bob_club", "member", "rbac", "principal", "bob", ""),
	}

	touch := api.TouchSemantics(false)

	_, err = spiceDbRepo.CreateRelationships(ctx, rels, touch, nil)
	assert.NoError(t, err)

	touch = true

	_, err = spiceDbRepo.CreateRelationships(ctx, rels, touch, nil)
	assert.NoError(t, err)

	container.WaitForQuantizationInterval()

	exists := CheckForRelationship(spiceDbRepo, "bob", "rbac", "principal", "", "member", "rbac", "group", "bob_club", nil)
	assert.True(t, exists)
}

// ============================================================================
// Bulk Import Test
// ============================================================================

// TestImportBulkTuples tests bulk tuple import
func TestImportBulkTuples(t *testing.T) {
	t.Skip("Requires gRPC streaming mock implementation - adapt when needed")
}

// ============================================================================
// Health Check Tests
// ============================================================================

// TestIsBackendAvailable tests successful backend health check
func TestIsBackendAvailable(t *testing.T) {
	t.Parallel()
	spiceDbRepo, err := container.CreateSpiceDbRepository()
	assert.NoError(t, err)

	err = spiceDbRepo.IsBackendAvailable()
	assert.NoError(t, err)
}

// TestIsBackendUnavailable tests backend health check failure
func TestIsBackendUnavailable(t *testing.T) {
	t.Parallel()
	logger := log.With(log.NewStdLogger(os.Stdout),
		"ts", log.DefaultTimestamp,
		"caller", log.DefaultCaller,
	)

	config := &Config{
		Endpoint:        "-1",
		Token:           "foobar",
		TokenFile:       "",
		SchemaFile:      "",
		UseTLS:          true,
		FullyConsistent: false,
	}

	spiceDbRepo, err := NewSpiceDbRepository(config, log.NewHelper(logger))
	assert.NoError(t, err)

	err = spiceDbRepo.IsBackendAvailable()
	assert.Error(t, err)
}

// ============================================================================
// Validation Tests
// ============================================================================

// TestDoesNotCreateRelationshipWithSlashInSubjectType verifies validation rejects slashes in subject type
func TestDoesNotCreateRelationshipWithSlashInSubjectType(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	spiceDbRepo, err := container.CreateSpiceDbRepository()
	assert.NoError(t, err)

	rels := []*kessel.Relationship{
		createRelationship("rbac", "group", "validation_group", "member", "rbac", "principal/invalid", "user1", ""),
	}

	_, err = spiceDbRepo.CreateRelationships(ctx, rels, api.TouchSemantics(false), nil)
	assert.Error(t, err)
}

// TestDoesNotCreateRelationshipWithSlashInObjectType verifies validation rejects slashes in object type
func TestDoesNotCreateRelationshipWithSlashInObjectType(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	spiceDbRepo, err := container.CreateSpiceDbRepository()
	assert.NoError(t, err)

	rels := []*kessel.Relationship{
		createRelationship("rbac", "group/invalid", "validation_group", "member", "rbac", "principal", "user1", ""),
	}

	_, err = spiceDbRepo.CreateRelationships(ctx, rels, api.TouchSemantics(false), nil)
	assert.Error(t, err)
}

// TestCreateRelationshipFailsWithBadSubjectType verifies error handling for invalid subject type
func TestCreateRelationshipFailsWithBadSubjectType(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	spiceDbRepo, err := container.CreateSpiceDbRepository()
	assert.NoError(t, err)

	rels := []*kessel.Relationship{
		createRelationship("rbac", "group", "validation_group", "member", "rbac", "nonexistent_type", "user1", ""),
	}

	_, err = spiceDbRepo.CreateRelationships(ctx, rels, api.TouchSemantics(false), nil)
	assert.Error(t, err)
}

// TestCreateRelationshipFailsWithBadObjectType verifies error handling for invalid object type
func TestCreateRelationshipFailsWithBadObjectType(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	spiceDbRepo, err := container.CreateSpiceDbRepository()
	assert.NoError(t, err)

	rels := []*kessel.Relationship{
		createRelationship("rbac", "nonexistent_type", "validation_resource", "member", "rbac", "principal", "user1", ""),
	}

	_, err = spiceDbRepo.CreateRelationships(ctx, rels, api.TouchSemantics(false), nil)
	assert.Error(t, err)
}

// ============================================================================
// Read/Delete Relationship Tests
// ============================================================================

// TestWriteAndReadBackRelationships tests reading back created relationships
func TestWriteAndReadBackRelationships(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	spiceDbRepo, err := container.CreateSpiceDbRepository()
	assert.NoError(t, err)

	rels := []*kessel.Relationship{
		createRelationship("rbac", "group", "read_test_group", "member", "rbac", "principal", "grace", ""),
		createRelationship("rbac", "group", "read_test_group", "member", "rbac", "principal", "henry", ""),
	}

	createResp, err := spiceDbRepo.CreateRelationships(ctx, rels, api.TouchSemantics(false), nil)
	assert.NoError(t, err)

	consistency := &kessel.Consistency{
		AtLeastAsFresh: createResp.ConsistencyToken,
	}

	// Read back all relationships for this group
	resourceNs := "rbac"
	resourceTp := "group"
	resourceId := "read_test_group"

	resultsChan, errChan, err := spiceDbRepo.ReadRelationships(ctx, &kessel.RelationTupleFilter{
		ResourceNamespace: &resourceNs,
		ResourceType:      &resourceTp,
		ResourceId:        &resourceId,
	}, 10, api.ContinuationToken(""), consistency)
	assert.NoError(t, err)

	results := spiceRelChanToSlice(resultsChan)
	err = <-errChan
	assert.NoError(t, err)

	assert.Len(t, results, 2)
}

// TestWriteReadBackDeleteAndReadBackRelationships tests delete operations
func TestWriteReadBackDeleteAndReadBackRelationships(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	spiceDbRepo, err := container.CreateSpiceDbRepository()
	assert.NoError(t, err)

	rels := []*kessel.Relationship{
		createRelationship("rbac", "group", "delete_test_group", "member", "rbac", "principal", "iris", ""),
	}

	createResp, err := spiceDbRepo.CreateRelationships(ctx, rels, api.TouchSemantics(false), nil)
	assert.NoError(t, err)

	consistency := &kessel.Consistency{
		AtLeastAsFresh: createResp.ConsistencyToken,
	}

	// Verify it exists
	exists := CheckForRelationship(spiceDbRepo, "iris", "rbac", "principal", "",
		"member", "rbac", "group", "delete_test_group", consistency)
	assert.True(t, exists)

	// Delete the relationship
	resourceNs := "rbac"
	resourceTp := "group"
	resourceId := "delete_test_group"
	relation := "member"
	subjectNs := "rbac"
	subjectTp := "principal"
	subjectId := "iris"

	deleteResp, err := spiceDbRepo.DeleteRelationships(ctx, &kessel.RelationTupleFilter{
		ResourceNamespace: &resourceNs,
		ResourceType:      &resourceTp,
		ResourceId:        &resourceId,
		Relation:          &relation,
		SubjectFilter: &kessel.SubjectFilter{
			SubjectNamespace: &subjectNs,
			SubjectType:      &subjectTp,
			SubjectId:        &subjectId,
		},
	}, nil)
	assert.NoError(t, err)

	deleteConsistency := &kessel.Consistency{
		AtLeastAsFresh: deleteResp.ConsistencyToken,
	}

	// Verify it's gone
	exists = CheckForRelationship(spiceDbRepo, "iris", "rbac", "principal", "",
		"member", "rbac", "group", "delete_test_group", deleteConsistency)
	assert.False(t, exists)
}

// TestWriteReadBackDeleteAndReadBackRelationships_WithConsistencyToken tests delete with consistency tokens
func TestWriteReadBackDeleteAndReadBackRelationships_WithConsistencyToken(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	spiceDbRepo, err := container.CreateSpiceDbRepository()
	assert.NoError(t, err)

	rels := []*kessel.Relationship{
		createRelationship("rbac", "group", "delete_consistency_group", "member", "rbac", "principal", "jack", ""),
	}

	createResp, err := spiceDbRepo.CreateRelationships(ctx, rels, api.TouchSemantics(false), nil)
	assert.NoError(t, err)

	consistency := &kessel.Consistency{
		AtLeastAsFresh: createResp.ConsistencyToken,
	}

	// Verify it exists with consistency token
	exists := CheckForRelationship(spiceDbRepo, "jack", "rbac", "principal", "",
		"member", "rbac", "group", "delete_consistency_group", consistency)
	assert.True(t, exists)

	// Delete
	resourceNs := "rbac"
	resourceTp := "group"
	resourceId := "delete_consistency_group"
	relation := "member"
	subjectNs := "rbac"
	subjectTp := "principal"
	subjectId := "jack"

	deleteResp, err := spiceDbRepo.DeleteRelationships(ctx, &kessel.RelationTupleFilter{
		ResourceNamespace: &resourceNs,
		ResourceType:      &resourceTp,
		ResourceId:        &resourceId,
		Relation:          &relation,
		SubjectFilter: &kessel.SubjectFilter{
			SubjectNamespace: &subjectNs,
			SubjectType:      &subjectTp,
			SubjectId:        &subjectId,
		},
	}, nil)
	assert.NoError(t, err)

	deleteConsistency := &kessel.Consistency{
		AtLeastAsFresh: deleteResp.ConsistencyToken,
	}

	// Verify it's gone with delete consistency token
	exists = CheckForRelationship(spiceDbRepo, "jack", "rbac", "principal", "",
		"member", "rbac", "group", "delete_consistency_group", deleteConsistency)
	assert.False(t, exists)
}

// TestSupportedNsTypeTupleFilterCombinationsInReadRelationships tests various filter combinations
func TestSupportedNsTypeTupleFilterCombinationsInReadRelationships(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	spiceDbRepo, err := container.CreateSpiceDbRepository()
	assert.NoError(t, err)

	// Create test data
	rels := []*kessel.Relationship{
		createRelationship("rbac", "group", "filter_test_group", "member", "rbac", "principal", "karen", ""),
	}
	createResp, err := spiceDbRepo.CreateRelationships(ctx, rels, api.TouchSemantics(false), nil)
	assert.NoError(t, err)

	consistency := &kessel.Consistency{
		AtLeastAsFresh: createResp.ConsistencyToken,
	}

	// Test filter with namespace and type
	resourceNs := "rbac"
	resourceTp := "group"

	resultsChan, errChan, err := spiceDbRepo.ReadRelationships(ctx, &kessel.RelationTupleFilter{
		ResourceNamespace: &resourceNs,
		ResourceType:      &resourceTp,
	}, 10, api.ContinuationToken(""), consistency)
	assert.NoError(t, err)

	results := spiceRelChanToSlice(resultsChan)
	err = <-errChan
	assert.NoError(t, err)

	assert.GreaterOrEqual(t, len(results), 1)
}

// ============================================================================
// Permission Check Tests
// ============================================================================

// TestSpiceDbRepository_CheckPermission tests basic permission checks
func TestSpiceDbRepository_CheckPermission(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	spiceDbRepo, err := container.CreateSpiceDbRepository()
	assert.NoError(t, err)

	// Create RBAC structure: role with permission, role_binding connecting user to role, widget in workspace
	rels := []*kessel.Relationship{
		// Role has view_widget permission
		createRelationship("rbac", "role", "viewer_role", "view_widget", "rbac", "principal", "*", ""),
		// Role binding grants role to user
		createRelationship("rbac", "role_binding", "leo_binding", "granted", "rbac", "role", "viewer_role", ""),
		createRelationship("rbac", "role_binding", "leo_binding", "subject", "rbac", "principal", "leo", ""),
		// Workspace has the role binding
		createRelationship("rbac", "workspace", "check_workspace", "user_grant", "rbac", "role_binding", "leo_binding", ""),
		// Widget belongs to workspace
		createRelationship("rbac", "widget", "check_widget", "workspace", "rbac", "workspace", "check_workspace", ""),
	}

	createResp, err := spiceDbRepo.CreateRelationships(ctx, rels, api.TouchSemantics(false), nil)
	assert.NoError(t, err)

	consistency := &kessel.Consistency{
		AtLeastAsFresh: createResp.ConsistencyToken,
	}

	// Check if leo can view the widget
	checkReq := &kessel.CheckRequest{
		Resource: &kessel.ObjectReference{
			Type: &kessel.ObjectType{Namespace: "rbac", Name: "widget"},
			Id:   "check_widget",
		},
		Relation: "view",
		Subject: &kessel.SubjectReference{
			Subject: &kessel.ObjectReference{
				Type: &kessel.ObjectType{Namespace: "rbac", Name: "principal"},
				Id:   "leo",
			},
		},
		Consistency: consistency,
	}

	checkResp, err := spiceDbRepo.Check(ctx, checkReq)
	assert.NoError(t, err)
	assert.Equal(t, kessel.AllowedTrue, checkResp.Allowed)
}

// TestSpiceDbRepository_CheckForUpdate tests fully consistent permission checks
func TestSpiceDbRepository_CheckForUpdate(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	spiceDbRepo, err := container.CreateSpiceDbRepository()
	assert.NoError(t, err)

	// Create relationship
	rels := []*kessel.Relationship{
		createRelationship("rbac", "group", "update_check_group", "member", "rbac", "principal", "maria", ""),
	}

	_, err = spiceDbRepo.CreateRelationships(ctx, rels, api.TouchSemantics(false), nil)
	assert.NoError(t, err)

	container.WaitForQuantizationInterval()

	// CheckForUpdate provides fully consistent reads
	checkReq := &kessel.CheckForUpdateRequest{
		Resource: &kessel.ObjectReference{
			Type: &kessel.ObjectType{Namespace: "rbac", Name: "group"},
			Id:   "update_check_group",
		},
		Relation: "member",
		Subject: &kessel.SubjectReference{
			Subject: &kessel.ObjectReference{
				Type: &kessel.ObjectType{Namespace: "rbac", Name: "principal"},
				Id:   "maria",
			},
		},
	}

	checkResp, err := spiceDbRepo.CheckForUpdate(ctx, checkReq)
	assert.NoError(t, err)
	assert.Equal(t, kessel.AllowedTrue, checkResp.Allowed)
}

// TestSpiceDbRepository_CheckPermission_MinimizeLatency tests eventual consistency checks
func TestSpiceDbRepository_CheckPermission_MinimizeLatency(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	spiceDbRepo, err := container.CreateSpiceDbRepository()
	assert.NoError(t, err)

	// Create relationship
	rels := []*kessel.Relationship{
		createRelationship("rbac", "group", "latency_check_group", "member", "rbac", "principal", "nancy", ""),
	}

	_, err = spiceDbRepo.CreateRelationships(ctx, rels, api.TouchSemantics(false), nil)
	assert.NoError(t, err)

	container.WaitForQuantizationInterval()

	// Check with minimize latency (eventual consistency)
	checkReq := &kessel.CheckRequest{
		Resource: &kessel.ObjectReference{
			Type: &kessel.ObjectType{Namespace: "rbac", Name: "group"},
			Id:   "latency_check_group",
		},
		Relation: "member",
		Subject: &kessel.SubjectReference{
			Subject: &kessel.ObjectReference{
				Type: &kessel.ObjectType{Namespace: "rbac", Name: "principal"},
				Id:   "nancy",
			},
		},
		Consistency: &kessel.Consistency{MinimizeLatency: true},
	}

	checkResp, err := spiceDbRepo.Check(ctx, checkReq)
	assert.NoError(t, err)
	assert.Equal(t, kessel.AllowedTrue, checkResp.Allowed)
}

// TestSpiceDbRepository_CheckPermission_WithConsistencyToken tests consistency token usage in checks
func TestSpiceDbRepository_CheckPermission_WithConsistencyToken(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	spiceDbRepo, err := container.CreateSpiceDbRepository()
	assert.NoError(t, err)

	// Create a relationship and get a consistency token
	rels := []*kessel.Relationship{
		createRelationship("rbac", "group", "consistency_group", "member", "rbac", "principal", "carol", ""),
	}
	createResp, err := spiceDbRepo.CreateRelationships(ctx, rels, api.TouchSemantics(false), nil)
	assert.NoError(t, err)
	consistencyToken := createResp.ConsistencyToken

	// Check permission with consistency token - should see the relationship immediately
	checkReq := &kessel.CheckRequest{
		Resource: &kessel.ObjectReference{
			Type: &kessel.ObjectType{Namespace: "rbac", Name: "group"},
			Id:   "consistency_group",
		},
		Relation: "member",
		Subject: &kessel.SubjectReference{
			Subject: &kessel.ObjectReference{
				Type: &kessel.ObjectType{Namespace: "rbac", Name: "principal"},
				Id:   "carol",
			},
		},
		Consistency: &kessel.Consistency{
			AtLeastAsFresh: consistencyToken,
		},
	}

	checkResp, err := spiceDbRepo.Check(ctx, checkReq)
	assert.NoError(t, err)
	assert.Equal(t, kessel.AllowedTrue, checkResp.Allowed, "Permission should be granted with consistency token")
}

// TestSpiceDbRepository_CheckBulk tests bulk permission checking
func TestSpiceDbRepository_CheckBulk(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	spiceDbRepo, err := container.CreateSpiceDbRepository()
	if !assert.NoError(t, err) {
		return
	}

	rels := []*kessel.Relationship{
		createRelationship("rbac", "group", "bob_club", "member", "rbac", "principal", "bob", ""),
		createRelationship("rbac", "workspace", "test", "user_grant", "rbac", "role_binding", "rb_test", ""),
		createRelationship("rbac", "role_binding", "rb_test", "granted", "rbac", "role", "rl1", ""),
		createRelationship("rbac", "role_binding", "rb_test", "subject", "rbac", "principal", "bob", ""),
		createRelationship("rbac", "role", "rl1", "view_widget", "rbac", "principal", "*", ""),
	}

	_, err = spiceDbRepo.CreateRelationships(ctx, rels, api.TouchSemantics(true), nil)
	if !assert.NoError(t, err) {
		return
	}

	container.WaitForQuantizationInterval()

	items := []*kessel.CheckBulkRequestItem{
		{
			Resource: &kessel.ObjectReference{
				Type: &kessel.ObjectType{
					Name:      "workspace",
					Namespace: "rbac",
				},
				Id: "test",
			},
			Relation: "view_widget",
			Subject: &kessel.SubjectReference{
				Subject: &kessel.ObjectReference{
					Type: &kessel.ObjectType{
						Name:      "principal",
						Namespace: "rbac",
					},
					Id: "bob",
				},
			},
		},
		{
			Resource: &kessel.ObjectReference{
				Type: &kessel.ObjectType{
					Name:      "workspace",
					Namespace: "rbac",
				},
				Id: "test",
			},
			Relation: "view_widget",
			Subject: &kessel.SubjectReference{
				Subject: &kessel.ObjectReference{
					Type: &kessel.ObjectType{
						Name:      "principal",
						Namespace: "rbac",
					},
					Id: "alice",
				},
			},
		},
	}

	req := &kessel.CheckBulkRequest{
		Items: items,
	}
	resp, err := spiceDbRepo.CheckBulk(ctx, req)
	if !assert.NoError(t, err) {
		return
	}

	if !assert.Equal(t, len(items), len(resp.Pairs)) {
		return
	}

	results := map[string]kessel.Allowed{}
	for _, p := range resp.Pairs {
		subjId := p.Request.Subject.Subject.Id
		results[subjId] = p.Item.Allowed
	}
	assert.Equal(t, kessel.AllowedTrue, results["bob"])
	assert.Equal(t, kessel.AllowedFalse, results["alice"])
}

// TestFromSpicePair_WithError tests error handling in bulk check responses
func TestFromSpicePair_WithError(t *testing.T) {
	t.Parallel()

	// Build a SpiceDB pair that contains an error instead of an item
	pair := &v1.CheckBulkPermissionsPair{
		Request: &v1.CheckBulkPermissionsRequestItem{
			Resource: &v1.ObjectReference{
				ObjectType: "rbac/workspace",
				ObjectId:   "test",
			},
			Permission: "view_widget",
			Subject: &v1.SubjectReference{
				Object: &v1.ObjectReference{
					ObjectType: "rbac/principal",
					ObjectId:   "bob",
				},
			},
		},
		Response: &v1.CheckBulkPermissionsPair_Error{
			Error: status.New(codes.InvalidArgument, "invalid request").Proto(),
		},
	}

	got := fromSpicePair(pair, log.NewHelper(log.DefaultLogger))
	assert.NotNil(t, got)
	// When error is present, the oneof response should be set to error and item should be nil
	assert.Nil(t, got.Item)
	if assert.NotNil(t, got.Error) {
		assert.Contains(t, got.Error.Error(), "invalid request")
	}

	// And the request should be preserved/mapped back correctly
	req := got.Request
	assert.Equal(t, "rbac", req.Resource.Type.Namespace)
	assert.Equal(t, "workspace", req.Resource.Type.Name)
	assert.Equal(t, "test", req.Resource.Id)
	assert.Equal(t, "view_widget", req.Relation)
	assert.Equal(t, "rbac", req.Subject.Subject.Type.Namespace)
	assert.Equal(t, "principal", req.Subject.Subject.Type.Name)
	assert.Equal(t, "bob", req.Subject.Subject.Id)
}

// TestSpiceDbRepository_NewEnemyProblem_Success tests complex transitive permission scenario
func TestSpiceDbRepository_NewEnemyProblem_Success(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	spiceDbRepo, err := container.CreateSpiceDbRepository()
	assert.NoError(t, err)

	// Setup: Create a workspace hierarchy with permissions
	// parent_workspace <- child_workspace <- widget
	// Role with use_widget permission granted to user in parent
	rels := []*kessel.Relationship{
		// Parent workspace setup
		createRelationship("rbac", "role", "enemy_role", "use_widget", "rbac", "principal", "*", ""),
		createRelationship("rbac", "role_binding", "enemy_binding", "granted", "rbac", "role", "enemy_role", ""),
		createRelationship("rbac", "role_binding", "enemy_binding", "subject", "rbac", "principal", "quinn", ""),
		createRelationship("rbac", "workspace", "parent_workspace", "user_grant", "rbac", "role_binding", "enemy_binding", ""),
		// Child workspace inherits from parent
		createRelationship("rbac", "workspace", "child_workspace", "parent", "rbac", "workspace", "parent_workspace", ""),
		// Widget in child workspace
		createRelationship("rbac", "widget", "enemy_widget", "workspace", "rbac", "workspace", "child_workspace", ""),
	}

	createResp, err := spiceDbRepo.CreateRelationships(ctx, rels, api.TouchSemantics(false), nil)
	assert.NoError(t, err)

	consistency := &kessel.Consistency{
		AtLeastAsFresh: createResp.ConsistencyToken,
	}

	// Quinn should have use permission on widget via parent workspace
	runSpiceDBCheck(t, ctx, spiceDbRepo,
		"principal", "rbac", "quinn",
		"use", "widget", "rbac", "enemy_widget",
		kessel.AllowedTrue, consistency)
}

// ============================================================================
// Fencing Tests
// ============================================================================

// TestSpiceDbRepository_CreateRelationships_WithFencing tests that fencing tokens
// prevent stale writes after lock acquisition
func TestSpiceDbRepository_CreateRelationships_WithFencing(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	spiceDbRepo, err := container.CreateSpiceDbRepository()
	assert.NoError(t, err)

	// Acquire a lock to get a fencing token
	lockIdentifier := "test-lock-1"
	lockResp, err := spiceDbRepo.AcquireLock(ctx, lockIdentifier)
	assert.NoError(t, err)
	fencingToken := lockResp.LockToken
	assert.NotEmpty(t, fencingToken)

	// Create a relationship with fencing
	rels := []*kessel.Relationship{
		createRelationship("rbac", "group", "fencing_group", "member", "rbac", "principal", "fenced_bob", ""),
	}
	touch := api.TouchSemantics(false)
	fencing := &kessel.FencingCheck{
		LockId:    lockIdentifier,
		LockToken: fencingToken,
	}
	_, err = spiceDbRepo.CreateRelationships(ctx, rels, touch, fencing)
	assert.NoError(t, err)

	container.WaitForQuantizationInterval()

	// Relationship should exist
	exists := CheckForRelationship(
		spiceDbRepo, "fenced_bob", "rbac", "principal", "", "member", "rbac", "group", "fencing_group", nil,
	)
	assert.True(t, exists)

	// Try to create with an invalid fencing token
	badFencing := &kessel.FencingCheck{
		LockId:    lockIdentifier,
		LockToken: "invalid-token",
	}
	_, err = spiceDbRepo.CreateRelationships(ctx, rels, touch, badFencing)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "error writing relationships to SpiceDB")

	// try to create with a non-existent lock id
	badFencing2 := &kessel.FencingCheck{
		LockId:    "invalid-lock-id",
		LockToken: fencingToken,
	}
	_, err = spiceDbRepo.CreateRelationships(ctx, rels, touch, badFencing2)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "error writing relationships to SpiceDB")
}

// TestSpiceDbRepository_DeleteRelationships_WithFencing tests fencing for delete operations
func TestSpiceDbRepository_DeleteRelationships_WithFencing(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	spiceDbRepo, err := container.CreateSpiceDbRepository()
	assert.NoError(t, err)

	// Create a relationship first
	rels := []*kessel.Relationship{
		createRelationship("rbac", "group", "fencing_delete_group", "member", "rbac", "principal", "rachel", ""),
	}
	createResp, err := spiceDbRepo.CreateRelationships(ctx, rels, api.TouchSemantics(false), nil)
	assert.NoError(t, err)

	consistency := &kessel.Consistency{
		AtLeastAsFresh: createResp.ConsistencyToken,
	}

	// Verify it exists
	exists := CheckForRelationship(spiceDbRepo, "rachel", "rbac", "principal", "",
		"member", "rbac", "group", "fencing_delete_group", consistency)
	assert.True(t, exists)

	// Acquire lock for fencing
	lockIdentifier := "delete-lock-1"
	lockResp, err := spiceDbRepo.AcquireLock(ctx, lockIdentifier)
	assert.NoError(t, err)
	fencingToken := lockResp.LockToken

	// Delete with valid fencing token
	resourceNs := "rbac"
	resourceTp := "group"
	resourceId := "fencing_delete_group"
	relation := "member"
	subjectNs := "rbac"
	subjectTp := "principal"
	subjectId := "rachel"

	fencing := &kessel.FencingCheck{
		LockId:    lockIdentifier,
		LockToken: fencingToken,
	}

	deleteResp, err := spiceDbRepo.DeleteRelationships(ctx, &kessel.RelationTupleFilter{
		ResourceNamespace: &resourceNs,
		ResourceType:      &resourceTp,
		ResourceId:        &resourceId,
		Relation:          &relation,
		SubjectFilter: &kessel.SubjectFilter{
			SubjectNamespace: &subjectNs,
			SubjectType:      &subjectTp,
			SubjectId:        &subjectId,
		},
	}, fencing)
	assert.NoError(t, err)

	deleteConsistency := &kessel.Consistency{
		AtLeastAsFresh: deleteResp.ConsistencyToken,
	}

	// Verify deletion succeeded
	exists = CheckForRelationship(spiceDbRepo, "rachel", "rbac", "principal", "",
		"member", "rbac", "group", "fencing_delete_group", deleteConsistency)
	assert.False(t, exists)

	// Try to delete again with invalid fencing token - should fail
	badFencing := &kessel.FencingCheck{
		LockId:    lockIdentifier,
		LockToken: "invalid-token",
	}
	_, err = spiceDbRepo.DeleteRelationships(ctx, &kessel.RelationTupleFilter{
		ResourceNamespace: &resourceNs,
		ResourceType:      &resourceTp,
		ResourceId:        &resourceId,
	}, badFencing)
	assert.Error(t, err)
}

// ============================================================================
// Lock Acquisition Tests
// ============================================================================

// TestSpiceDbRepository_AcquireLock_NewLock tests acquiring a new lock
func TestSpiceDbRepository_AcquireLock_NewLock(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	spiceDbRepo, err := container.CreateSpiceDbRepository()
	assert.NoError(t, err)

	lockIdentifier := "test-new-lock"
	lockResp, err := spiceDbRepo.AcquireLock(ctx, lockIdentifier)
	assert.NoError(t, err)
	assert.NotEmpty(t, lockResp.LockToken)
}

// TestSpiceDbRepository_AcquireLock_ReplaceExistingLock tests that acquiring a lock
// a second time invalidates the previous token
func TestSpiceDbRepository_AcquireLock_ReplaceExistingLock(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	spiceDbRepo, err := container.CreateSpiceDbRepository()
	assert.NoError(t, err)

	lockIdentifier := "test-replace-lock"

	// First lock acquisition
	lockResp1, err := spiceDbRepo.AcquireLock(ctx, lockIdentifier)
	assert.NoError(t, err)
	token1 := lockResp1.LockToken
	assert.NotEmpty(t, token1)

	// Second lock acquisition should replace the first
	lockResp2, err := spiceDbRepo.AcquireLock(ctx, lockIdentifier)
	assert.NoError(t, err)
	token2 := lockResp2.LockToken
	assert.NotEmpty(t, token2)
	assert.NotEqual(t, token1, token2, "Second lock acquisition should generate a different token")

	// Create relationship with first (now stale) token - should fail
	rels := []*kessel.Relationship{
		createRelationship("rbac", "group", "lock_test_group", "member", "rbac", "principal", "alice", ""),
	}
	fencing1 := &kessel.FencingCheck{
		LockId:    lockIdentifier,
		LockToken: token1,
	}
	_, err = spiceDbRepo.CreateRelationships(ctx, rels, api.TouchSemantics(false), fencing1)
	assert.Error(t, err, "Using stale fencing token should fail")

	// Create with second (current) token - should succeed
	fencing2 := &kessel.FencingCheck{
		LockId:    lockIdentifier,
		LockToken: token2,
	}
	_, err = spiceDbRepo.CreateRelationships(ctx, rels, api.TouchSemantics(false), fencing2)
	assert.NoError(t, err, "Using current fencing token should succeed")
}

// TestSpiceDbRepository_AcquireLock_EmptyIdentifier tests error handling for empty lock ID
func TestSpiceDbRepository_AcquireLock_EmptyIdentifier(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	spiceDbRepo, err := container.CreateSpiceDbRepository()
	assert.NoError(t, err)

	_, err = spiceDbRepo.AcquireLock(ctx, "")
	assert.Error(t, err)
}

// ============================================================================
// Helper Functions
// ============================================================================

// createRelationship is a helper to create relationship objects for testing
func createRelationship(resourceNamespace, resourceType, resourceId, relation, subjectNamespace, subjectType, subjectId, subjectRelation string) *kessel.Relationship {
	var subjectRel *string
	if subjectRelation != "" {
		subjectRel = &subjectRelation
	}

	return &kessel.Relationship{
		Resource: &kessel.ObjectReference{
			Type: &kessel.ObjectType{
				Namespace: resourceNamespace,
				Name:      resourceType,
			},
			Id: resourceId,
		},
		Relation: relation,
		Subject: &kessel.SubjectReference{
			Subject: &kessel.ObjectReference{
				Type: &kessel.ObjectType{
					Namespace: subjectNamespace,
					Name:      subjectType,
				},
				Id: subjectId,
			},
			Relation: subjectRel,
		},
	}
}

// CheckForRelationship returns true if the given subject has the given relationship to the given resource
func CheckForRelationship(
	repo *SpiceDbRepository,
	subjectID, subjectNamespace, subjectType, subjectRelationship, relation, resourceNamespace, resourceType, resourceID string,
	consistency *kessel.Consistency,
) bool {
	ctx := context.TODO()

	var subjectRelationRef *string
	if subjectRelationship != "" {
		subjectRelationRef = &subjectRelationship
	}

	resourceNs := resourceNamespace
	resourceTp := resourceType
	resourceId := resourceID
	rel := relation
	subjectNs := subjectNamespace
	subjectTp := subjectType
	subjectId := subjectID

	results, errors, err := repo.ReadRelationships(ctx, &kessel.RelationTupleFilter{
		ResourceNamespace: &resourceNs,
		ResourceType:      &resourceTp,
		ResourceId:        &resourceId,
		Relation:          &rel,
		SubjectFilter: &kessel.SubjectFilter{
			SubjectNamespace: &subjectNs,
			SubjectType:      &subjectTp,
			SubjectId:        &subjectId,
			Relation:         subjectRelationRef,
		},
	}, 1, api.ContinuationToken(""), consistency)

	if err != nil {
		panic(err)
	}

	found := false
	select {
	case err, ok := <-errors:
		if ok {
			panic(err)
		}
	case _, ok := <-results:
		if ok {
			found = true
		}
	}

	return found
}

// runSpiceDBCheck is a helper that runs a permission check and asserts the expected result
func runSpiceDBCheck(t *testing.T, ctx context.Context, spiceDbRepo *SpiceDbRepository,
	subjectType, subjectNamespace, subjectID, relation, resourceType, resourceNamespace, resourceID string,
	expectedAllowed kessel.Allowed, consistency *kessel.Consistency) {

	checkReq := &kessel.CheckRequest{
		Resource: &kessel.ObjectReference{
			Type: &kessel.ObjectType{Namespace: resourceNamespace, Name: resourceType},
			Id:   resourceID,
		},
		Relation: relation,
		Subject: &kessel.SubjectReference{
			Subject: &kessel.ObjectReference{
				Type: &kessel.ObjectType{Namespace: subjectNamespace, Name: subjectType},
				Id:   subjectID,
			},
		},
		Consistency: consistency,
	}

	checkResp, err := spiceDbRepo.Check(ctx, checkReq)
	assert.NoError(t, err)
	assert.Equal(t, expectedAllowed, checkResp.Allowed)
}

// pointerize converts a string to a pointer
func pointerize(value string) *string {
	return &value
}

// spiceRelChanToSlice converts a channel of relationship results to a slice
func spiceRelChanToSlice(c chan *api.RelationshipResult) []*api.RelationshipResult {
	var results []*api.RelationshipResult
	for result := range c {
		results = append(results, result)
	}
	return results
}
