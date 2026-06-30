package model_test

import (
	"testing"

	"github.com/project-kessel/inventory-api/internal/biz/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestReporterResourceKey(t *testing.T) model.ReporterResourceKey {
	t.Helper()
	resourceType, err := model.NewResourceType("service")
	require.NoError(t, err)
	reporterType, err := model.NewReporterType("features")
	require.NoError(t, err)
	reporterInstanceId, err := model.NewReporterInstanceId("test-instance")
	require.NoError(t, err)
	key, err := model.NewReporterResourceKey(
		model.DeserializeLocalResourceId("svc-123"),
		resourceType, reporterType, reporterInstanceId,
	)
	require.NoError(t, err)
	return key
}

func TestNewRelationTupleForSubject(t *testing.T) {
	key := newTestReporterResourceKey(t)

	tests := []struct {
		name                string
		relationName        string
		subjectNamespace    string
		subjectResourceType string
		subjectId           string
	}{
		{
			name:                "workspace relation",
			relationName:        "allowed_workspaces",
			subjectNamespace:    "rbac",
			subjectResourceType: "workspace",
			subjectId:           "ws-001",
		},
		{
			name:                "billing account relation",
			relationName:        "billing_account",
			subjectNamespace:    "features",
			subjectResourceType: "billing_account",
			subjectId:           "ba-001",
		},
		{
			name:                "parent service relation",
			relationName:        "parent",
			subjectNamespace:    "features",
			subjectResourceType: "service",
			subjectId:           "parent-svc-001",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tuple := model.NewRelationTupleForSubject(
				key, tt.relationName, tt.subjectNamespace, tt.subjectResourceType, tt.subjectId,
			)

			object := tuple.Object()
			assert.Equal(t, "service", object.ResourceType().Serialize())
			assert.Equal(t, "svc-123", object.ResourceId().Serialize())
			assert.True(t, object.HasReporter())
			assert.Equal(t, "features", object.Reporter().ReporterType().Serialize())

			assert.Equal(t, tt.relationName, tuple.Relation().Serialize())

			subject := tuple.Subject().Resource()
			assert.Equal(t, tt.subjectResourceType, subject.ResourceType().Serialize())
			assert.Equal(t, tt.subjectId, subject.ResourceId().Serialize())
			assert.True(t, subject.HasReporter())
			assert.Equal(t, tt.subjectNamespace, subject.Reporter().ReporterType().Serialize())
		})
	}
}

func TestNewRelationTupleForSubject_MatchesWorkspaceTuple(t *testing.T) {
	resourceType, err := model.NewResourceType("host")
	require.NoError(t, err)
	reporterType, err := model.NewReporterType("hbi")
	require.NoError(t, err)
	reporterInstanceId, err := model.NewReporterInstanceId("test-instance")
	require.NoError(t, err)
	key, err := model.NewReporterResourceKey(
		model.DeserializeLocalResourceId("test-resource"),
		resourceType, reporterType, reporterInstanceId,
	)
	require.NoError(t, err)

	workspaceTuple := model.NewWorkspaceRelationsTuple("ws-123", key)
	genericTuple := model.NewRelationTupleForSubject(key, "workspace", "rbac", "workspace", "ws-123")

	assert.Equal(t, workspaceTuple.Object().ResourceType(), genericTuple.Object().ResourceType())
	assert.Equal(t, workspaceTuple.Object().ResourceId(), genericTuple.Object().ResourceId())
	assert.Equal(t, workspaceTuple.Relation(), genericTuple.Relation())
	assert.Equal(t, workspaceTuple.Subject().Resource().ResourceType(), genericTuple.Subject().Resource().ResourceType())
	assert.Equal(t, workspaceTuple.Subject().Resource().ResourceId(), genericTuple.Subject().Resource().ResourceId())
}
