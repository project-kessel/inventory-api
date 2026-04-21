package data

import (
	"context"
	"io"
	"sort"
	"testing"

	"github.com/project-kessel/inventory-api/internal/biz/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testResourceKey(namespace, resourceType, resourceID string) model.ReporterResourceKey {
	key, _ := model.NewReporterResourceKey(
		model.DeserializeLocalResourceId(resourceID),
		model.DeserializeResourceType(resourceType),
		model.DeserializeReporterType(namespace),
		model.DeserializeReporterInstanceId(""),
	)
	return key
}

func testSubjectRef(subjectID string) model.SubjectReference {
	key, _ := model.NewReporterResourceKey(
		model.DeserializeLocalResourceId(subjectID),
		model.DeserializeResourceType("principal"),
		model.DeserializeReporterType("rbac"),
		model.DeserializeReporterInstanceId(""),
	)
	return model.NewSubjectReferenceWithoutRelation(key)
}

func testModelCheckBulkItem(namespace, resourceType, resourceID, relation, subjectID string) model.CheckBulkItem {
	return model.CheckBulkItem{
		Resource: testResourceKey(namespace, resourceType, resourceID),
		Relation: model.DeserializeRelation(relation),
		Subject:  testSubjectRef(subjectID),
	}
}

func testPrincipalTuple(namespace, resourceType, resourceID, relation, subjectID string) model.RelationsTuple {
	resourceObjType := model.NewRelationsObjectType(resourceType, namespace)
	resource := model.NewRelationsResource(model.DeserializeLocalResourceId(resourceID), resourceObjType)
	subjectObjType := model.NewRelationsObjectType("principal", "rbac")
	subjectResource := model.NewRelationsResource(model.DeserializeLocalResourceId(subjectID), subjectObjType)
	subject := model.NewRelationsSubject(subjectResource, nil)

	return model.NewRelationsTuple(resource, model.DeserializeRelation(relation), subject)
}

func TestSimpleRelationsRepository_DefaultDeny(t *testing.T) {
	repo := NewSimpleRelationsRepository()

	result, err := repo.Check(context.Background(),
		testResourceKey("hbi", "host", "resource-1"),
		model.DeserializeRelation("view"),
		testSubjectRef("user-123"),
		model.NewConsistencyMinimizeLatency(),
	)

	require.NoError(t, err)
	assert.False(t, result.Allowed)
}

func TestSimpleRelationsRepository_CreateTuples_ThenCheck(t *testing.T) {
	repo := NewSimpleRelationsRepository()

	tuples := []model.RelationsTuple{
		testPrincipalTuple("hbi", "host", "resource-1", "view", "user-123"),
	}

	_, err := repo.CreateTuples(context.Background(), tuples, false, nil)
	require.NoError(t, err)

	result, err := repo.Check(context.Background(),
		testResourceKey("hbi", "host", "resource-1"),
		model.DeserializeRelation("view"),
		testSubjectRef("user-123"),
		model.NewConsistencyMinimizeLatency(),
	)

	require.NoError(t, err)
	assert.True(t, result.Allowed)
}

func TestSimpleRelationsRepository_DeleteTuples(t *testing.T) {
	repo := NewSimpleRelationsRepository()

	tuples := []model.RelationsTuple{
		testPrincipalTuple("hbi", "host", "resource-1", "view", "user-123"),
	}
	_, _ = repo.CreateTuples(context.Background(), tuples, false, nil)

	_, err := repo.DeleteTuples(context.Background(), tuples, nil)
	require.NoError(t, err)

	result, err := repo.Check(context.Background(),
		testResourceKey("hbi", "host", "resource-1"),
		model.DeserializeRelation("view"),
		testSubjectRef("user-123"),
		model.NewConsistencyMinimizeLatency(),
	)

	require.NoError(t, err)
	assert.False(t, result.Allowed)
}

func TestSimpleRelationsRepository_Grant(t *testing.T) {
	repo := NewSimpleRelationsRepository()
	repo.Grant("user-123", "view", "hbi", "host", "resource-1")

	result, err := repo.Check(context.Background(),
		testResourceKey("hbi", "host", "resource-1"),
		model.DeserializeRelation("view"),
		testSubjectRef("user-123"),
		model.NewConsistencyMinimizeLatency(),
	)

	require.NoError(t, err)
	assert.True(t, result.Allowed)
}

func TestSimpleRelationsRepository_Health(t *testing.T) {
	repo := NewSimpleRelationsRepository()

	resp, err := repo.Health(context.Background())

	require.NoError(t, err)
	assert.Equal(t, "OK", resp.Status)
}

func TestSimpleRelationsRepository_Version(t *testing.T) {
	repo := NewSimpleRelationsRepository()
	assert.Equal(t, int64(1), repo.Version())

	repo.Grant("user-a", "view", "hbi", "host", "resource-1")
	assert.Equal(t, int64(2), repo.Version())
}

func TestSimpleRelationsRepository_ConsistencyToken(t *testing.T) {
	repo := NewSimpleRelationsRepository()

	result, err := repo.Check(context.Background(),
		testResourceKey("hbi", "host", "resource-1"),
		model.DeserializeRelation("view"),
		testSubjectRef("user-a"),
		model.NewConsistencyMinimizeLatency(),
	)
	require.NoError(t, err)
	assert.Equal(t, "1", result.ConsistencyToken.Serialize())

	repo.Grant("user-a", "view", "hbi", "host", "resource-1")

	result, err = repo.Check(context.Background(),
		testResourceKey("hbi", "host", "resource-1"),
		model.DeserializeRelation("view"),
		testSubjectRef("user-a"),
		model.NewConsistencyMinimizeLatency(),
	)
	require.NoError(t, err)
	assert.Equal(t, "2", result.ConsistencyToken.Serialize())
}

func TestSimpleRelationsRepository_RequiresExactMatch(t *testing.T) {
	repo := NewSimpleRelationsRepository()
	repo.Grant("user-123", "view", "hbi", "host", "resource-1")

	tests := []struct {
		name         string
		namespace    string
		permission   string
		resourceType string
		resourceID   string
		subjectID    string
		subjectNS    string
		subjectType  string
	}{
		{"wrong subject", "hbi", "view", "host", "resource-1", "other-user", "rbac", "principal"},
		{"wrong permission", "hbi", "edit", "host", "resource-1", "user-123", "rbac", "principal"},
		{"wrong namespace", "other", "view", "host", "resource-1", "user-123", "rbac", "principal"},
		{"wrong resource type", "hbi", "view", "vm", "resource-1", "user-123", "rbac", "principal"},
		{"wrong resource id", "hbi", "view", "host", "resource-2", "user-123", "rbac", "principal"},
		{"wrong subject namespace", "hbi", "view", "host", "resource-1", "user-123", "other", "principal"},
		{"wrong subject type", "hbi", "view", "host", "resource-1", "user-123", "rbac", "group"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			subKey, _ := model.NewReporterResourceKey(
				model.DeserializeLocalResourceId(tt.subjectID),
				model.DeserializeResourceType(tt.subjectType),
				model.DeserializeReporterType(tt.subjectNS),
				model.DeserializeReporterInstanceId(""),
			)
			sub := model.NewSubjectReferenceWithoutRelation(subKey)
			result, err := repo.Check(context.Background(),
				testResourceKey(tt.namespace, tt.resourceType, tt.resourceID),
				model.DeserializeRelation(tt.permission),
				sub,
				model.NewConsistencyMinimizeLatency(),
			)
			require.NoError(t, err)
			assert.False(t, result.Allowed)
		})
	}
}

func TestSimpleRelationsRepository_Reset(t *testing.T) {
	repo := NewSimpleRelationsRepository()
	repo.Grant("user-123", "view", "hbi", "host", "resource-1")

	result, err := repo.Check(context.Background(),
		testResourceKey("hbi", "host", "resource-1"),
		model.DeserializeRelation("view"),
		testSubjectRef("user-123"),
		model.NewConsistencyMinimizeLatency(),
	)
	require.NoError(t, err)
	assert.True(t, result.Allowed)

	repo.Reset()

	result, err = repo.Check(context.Background(),
		testResourceKey("hbi", "host", "resource-1"),
		model.DeserializeRelation("view"),
		testSubjectRef("user-123"),
		model.NewConsistencyMinimizeLatency(),
	)
	require.NoError(t, err)
	assert.False(t, result.Allowed)
}

func TestSimpleRelationsRepository_CheckForUpdate(t *testing.T) {
	repo := NewSimpleRelationsRepository()
	repo.Grant("user-123", "edit", "hbi", "host", "resource-1")

	result, err := repo.CheckForUpdate(context.Background(),
		testResourceKey("hbi", "host", "resource-1"),
		model.DeserializeRelation("edit"),
		testSubjectRef("user-123"),
	)
	require.NoError(t, err)
	assert.True(t, result.Allowed)
}

func TestSimpleRelationsRepository_CheckBulk(t *testing.T) {
	repo := NewSimpleRelationsRepository()
	repo.Grant("user-123", "view", "hbi", "host", "resource-1")

	items := []model.CheckBulkItem{
		testModelCheckBulkItem("hbi", "host", "resource-1", "view", "user-123"),
		testModelCheckBulkItem("hbi", "host", "resource-2", "view", "user-123"),
	}
	result, err := repo.CheckBulk(context.Background(), items, model.NewConsistencyMinimizeLatency())
	require.NoError(t, err)
	require.Len(t, result.Pairs, 2)
	assert.True(t, result.Pairs[0].Result.Allowed)
	assert.False(t, result.Pairs[1].Result.Allowed)
}

func TestSimpleRelationsRepository_CheckForUpdateBulk(t *testing.T) {
	repo := NewSimpleRelationsRepository()
	repo.Grant("user-123", "view", "hbi", "host", "resource-1")

	items := []model.CheckBulkItem{
		testModelCheckBulkItem("hbi", "host", "resource-1", "view", "user-123"),
		testModelCheckBulkItem("hbi", "host", "resource-2", "view", "user-123"),
	}
	result, err := repo.CheckForUpdateBulk(context.Background(), items)
	require.NoError(t, err)
	require.Len(t, result.Pairs, 2)
	assert.True(t, result.Pairs[0].Result.Allowed)
	assert.False(t, result.Pairs[1].Result.Allowed)
	assert.NotEmpty(t, result.ConsistencyToken.Serialize())
}

func TestSimpleRelationsRepository_MultipleTuples(t *testing.T) {
	repo := NewSimpleRelationsRepository()

	tuples := []model.RelationsTuple{
		testPrincipalTuple("hbi", "host", "resource-1", "view", "user-a"),
		testPrincipalTuple("hbi", "host", "resource-2", "view", "user-b"),
	}
	_, err := repo.CreateTuples(context.Background(), tuples, false, nil)
	require.NoError(t, err)

	result, err := repo.Check(context.Background(),
		testResourceKey("hbi", "host", "resource-1"),
		model.DeserializeRelation("view"),
		testSubjectRef("user-a"),
		model.NewConsistencyMinimizeLatency(),
	)
	require.NoError(t, err)
	assert.True(t, result.Allowed)

	result, err = repo.Check(context.Background(),
		testResourceKey("hbi", "host", "resource-2"),
		model.DeserializeRelation("view"),
		testSubjectRef("user-b"),
		model.NewConsistencyMinimizeLatency(),
	)
	require.NoError(t, err)
	assert.True(t, result.Allowed)
}

func TestSimpleRelationsRepository_LookupResources(t *testing.T) {
	repo := NewSimpleRelationsRepository()
	repo.Grant("user-a", "view", "hbi", "host", "resource-1")
	repo.Grant("user-a", "view", "hbi", "host", "resource-2")
	repo.Grant("user-b", "view", "hbi", "host", "resource-3")

	stream, err := repo.LookupResources(context.Background(),
		model.DeserializeResourceType("host"),
		model.DeserializeReporterType("hbi"),
		model.DeserializeRelation("view"),
		testSubjectRef("user-a"),
		nil,
		model.NewConsistencyMinimizeLatency(),
	)
	require.NoError(t, err)

	var ids []string
	for {
		item, recvErr := stream.Recv()
		if recvErr == io.EOF {
			break
		}
		require.NoError(t, recvErr)
		ids = append(ids, item.ResourceId.String())
	}
	sort.Strings(ids)
	assert.Equal(t, []string{"resource-1", "resource-2"}, ids)
}

func TestSimpleRelationsRepository_LookupResources_Empty(t *testing.T) {
	repo := NewSimpleRelationsRepository()

	stream, err := repo.LookupResources(context.Background(),
		model.DeserializeResourceType("host"),
		model.DeserializeReporterType("hbi"),
		model.DeserializeRelation("view"),
		testSubjectRef("user-a"),
		nil,
		model.NewConsistencyMinimizeLatency(),
	)
	require.NoError(t, err)

	_, err = stream.Recv()
	require.ErrorIs(t, err, io.EOF)
}

func TestSimpleRelationsRepository_LookupSubjects(t *testing.T) {
	repo := NewSimpleRelationsRepository()
	repo.Grant("user-a", "view", "hbi", "host", "resource-1")
	repo.Grant("user-b", "view", "hbi", "host", "resource-1")
	repo.Grant("user-c", "view", "hbi", "host", "resource-2")

	stream, err := repo.LookupSubjects(context.Background(),
		testResourceKey("hbi", "host", "resource-1"),
		model.DeserializeRelation("view"),
		model.DeserializeResourceType("principal"),
		model.DeserializeReporterType("rbac"),
		nil, nil,
		model.NewConsistencyMinimizeLatency(),
	)
	require.NoError(t, err)

	var ids []string
	for {
		item, recvErr := stream.Recv()
		if recvErr == io.EOF {
			break
		}
		require.NoError(t, recvErr)
		ids = append(ids, item.SubjectId.String())
	}
	sort.Strings(ids)
	assert.Equal(t, []string{"user-a", "user-b"}, ids)
}

func TestSimpleRelationsRepository_Version_AdvancesOnMutations(t *testing.T) {
	repo := NewSimpleRelationsRepository()
	assert.Equal(t, int64(1), repo.Version())

	repo.Grant("user-a", "view", "hbi", "host", "resource-1")
	assert.Equal(t, int64(2), repo.Version())

	tuples := []model.RelationsTuple{
		testPrincipalTuple("hbi", "host", "resource-2", "view", "user-b"),
	}
	_, err := repo.CreateTuples(context.Background(), tuples, false, nil)
	require.NoError(t, err)
	assert.Equal(t, int64(3), repo.Version())

	_, err = repo.DeleteTuples(context.Background(), tuples, nil)
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

	tuples := []model.RelationsTuple{
		testPrincipalTuple("hbi", "host", "resource-1", "view", "user-a"),
	}
	_, err := repo.DeleteTuples(context.Background(), tuples, nil)
	require.NoError(t, err)
	assert.Equal(t, int64(3), repo.Version())

	// Without token: uses oldest available snapshot (v2, retained) -> allowed
	result, err := repo.Check(context.Background(),
		testResourceKey("hbi", "host", "resource-1"),
		model.DeserializeRelation("view"),
		testSubjectRef("user-a"),
		model.NewConsistencyMinimizeLatency(),
	)
	require.NoError(t, err)
	assert.True(t, result.Allowed)

	// With token at v3 (current): deleted -> denied
	token3 := model.DeserializeConsistencyToken(simpleFormatConsistencyToken(3))
	result, err = repo.Check(context.Background(),
		testResourceKey("hbi", "host", "resource-1"),
		model.DeserializeRelation("view"),
		testSubjectRef("user-a"),
		model.NewConsistencyAtLeastAsFresh(token3),
	)
	require.NoError(t, err)
	assert.False(t, result.Allowed)

	// With token at v2 (snapshot): still has tuple -> allowed
	token2 := model.DeserializeConsistencyToken(simpleFormatConsistencyToken(2))
	result, err = repo.Check(context.Background(),
		testResourceKey("hbi", "host", "resource-1"),
		model.DeserializeRelation("view"),
		testSubjectRef("user-a"),
		model.NewConsistencyAtLeastAsFresh(token2),
	)
	require.NoError(t, err)
	assert.True(t, result.Allowed)
}

func TestSimpleRelationsRepository_Snapshot_NoRetained_UsesLatest(t *testing.T) {
	repo := NewSimpleRelationsRepository()
	repo.Grant("user-a", "view", "hbi", "host", "resource-1")
	assert.Equal(t, int64(2), repo.Version())

	token1 := model.DeserializeConsistencyToken(simpleFormatConsistencyToken(1))
	result, err := repo.Check(context.Background(),
		testResourceKey("hbi", "host", "resource-1"),
		model.DeserializeRelation("view"),
		testSubjectRef("user-a"),
		model.NewConsistencyAtLeastAsFresh(token1),
	)
	require.NoError(t, err)
	assert.True(t, result.Allowed)
}

func TestSimpleRelationsRepository_Snapshot_FindsOldestAtLeastAsFresh(t *testing.T) {
	repo := NewSimpleRelationsRepository()
	repo.Grant("user-a", "view", "hbi", "host", "resource-1")
	repo.RetainCurrentSnapshot()

	repo.Grant("user-a", "view", "hbi", "host", "resource-2")
	repo.RetainCurrentSnapshot()

	tuples := []model.RelationsTuple{
		testPrincipalTuple("hbi", "host", "resource-1", "view", "user-a"),
	}
	_, err := repo.DeleteTuples(context.Background(), tuples, nil)
	require.NoError(t, err)
	assert.Equal(t, int64(4), repo.Version())

	token2 := model.DeserializeConsistencyToken(simpleFormatConsistencyToken(2))
	result, err := repo.Check(context.Background(),
		testResourceKey("hbi", "host", "resource-1"),
		model.DeserializeRelation("view"),
		testSubjectRef("user-a"),
		model.NewConsistencyAtLeastAsFresh(token2),
	)
	require.NoError(t, err)
	assert.True(t, result.Allowed)

	token3 := model.DeserializeConsistencyToken(simpleFormatConsistencyToken(3))
	result, err = repo.Check(context.Background(),
		testResourceKey("hbi", "host", "resource-1"),
		model.DeserializeRelation("view"),
		testSubjectRef("user-a"),
		model.NewConsistencyAtLeastAsFresh(token3),
	)
	require.NoError(t, err)
	assert.True(t, result.Allowed)

	token4 := model.DeserializeConsistencyToken(simpleFormatConsistencyToken(4))
	result, err = repo.Check(context.Background(),
		testResourceKey("hbi", "host", "resource-1"),
		model.DeserializeRelation("view"),
		testSubjectRef("user-a"),
		model.NewConsistencyAtLeastAsFresh(token4),
	)
	require.NoError(t, err)
	assert.False(t, result.Allowed)

	result, err = repo.Check(context.Background(),
		testResourceKey("hbi", "host", "resource-2"),
		model.DeserializeRelation("view"),
		testSubjectRef("user-a"),
		model.NewConsistencyAtLeastAsFresh(token4),
	)
	require.NoError(t, err)
	assert.True(t, result.Allowed)
}

func TestSimpleRelationsRepository_Snapshot_Release(t *testing.T) {
	repo := NewSimpleRelationsRepository()
	repo.Grant("user-a", "view", "hbi", "host", "resource-1")
	v := repo.RetainCurrentSnapshot()

	tuples := []model.RelationsTuple{
		testPrincipalTuple("hbi", "host", "resource-1", "view", "user-a"),
	}
	_, err := repo.DeleteTuples(context.Background(), tuples, nil)
	require.NoError(t, err)

	tokenV := model.DeserializeConsistencyToken(simpleFormatConsistencyToken(v))
	result, err := repo.Check(context.Background(),
		testResourceKey("hbi", "host", "resource-1"),
		model.DeserializeRelation("view"),
		testSubjectRef("user-a"),
		model.NewConsistencyAtLeastAsFresh(tokenV),
	)
	require.NoError(t, err)
	assert.True(t, result.Allowed)

	repo.ReleaseSnapshot(v)

	result, err = repo.Check(context.Background(),
		testResourceKey("hbi", "host", "resource-1"),
		model.DeserializeRelation("view"),
		testSubjectRef("user-a"),
		model.NewConsistencyAtLeastAsFresh(tokenV),
	)
	require.NoError(t, err)
	assert.False(t, result.Allowed)
}

func TestSimpleRelationsRepository_Snapshot_ClearAll(t *testing.T) {
	repo := NewSimpleRelationsRepository()
	repo.Grant("user-a", "view", "hbi", "host", "resource-1")
	repo.RetainCurrentSnapshot()
	repo.Grant("user-a", "view", "hbi", "host", "resource-2")
	repo.RetainCurrentSnapshot()

	repo.ClearSnapshots()

	token2 := model.DeserializeConsistencyToken(simpleFormatConsistencyToken(2))
	result, err := repo.Check(context.Background(),
		testResourceKey("hbi", "host", "resource-1"),
		model.DeserializeRelation("view"),
		testSubjectRef("user-a"),
		model.NewConsistencyAtLeastAsFresh(token2),
	)
	require.NoError(t, err)
	assert.True(t, result.Allowed)
}

func TestSimpleRelationsRepository_CheckBulk_WithConsistencyToken(t *testing.T) {
	repo := NewSimpleRelationsRepository()
	repo.Grant("user-a", "view", "hbi", "host", "resource-1")
	repo.RetainCurrentSnapshot()

	tuples := []model.RelationsTuple{
		testPrincipalTuple("hbi", "host", "resource-1", "view", "user-a"),
	}
	_, err := repo.DeleteTuples(context.Background(), tuples, nil)
	require.NoError(t, err)

	item := testModelCheckBulkItem("hbi", "host", "resource-1", "view", "user-a")

	// No consistency token -> uses oldest available snapshot -> allowed
	respOldest, err := repo.CheckBulk(context.Background(), []model.CheckBulkItem{item}, model.NewConsistencyMinimizeLatency())
	require.NoError(t, err)
	assert.True(t, respOldest.Pairs[0].Result.Allowed)

	// At latest version -> denied
	tokenLatest := model.DeserializeConsistencyToken(simpleFormatConsistencyToken(repo.Version()))
	respLatest, err := repo.CheckBulk(context.Background(), []model.CheckBulkItem{item}, model.NewConsistencyAtLeastAsFresh(tokenLatest))
	require.NoError(t, err)
	assert.False(t, respLatest.Pairs[0].Result.Allowed)

	// At snapshot v2 -> allowed
	token2 := model.DeserializeConsistencyToken(simpleFormatConsistencyToken(2))
	respSnap, err := repo.CheckBulk(context.Background(), []model.CheckBulkItem{item}, model.NewConsistencyAtLeastAsFresh(token2))
	require.NoError(t, err)
	assert.True(t, respSnap.Pairs[0].Result.Allowed)
}
