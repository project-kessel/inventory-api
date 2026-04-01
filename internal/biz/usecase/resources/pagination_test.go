package resources

import (
	"testing"

	"github.com/project-kessel/inventory-api/internal/biz/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPaginationToV1beta1_NilInput(t *testing.T) {
	var pagination *model.Pagination = nil

	result := paginationToV1beta1(pagination)

	assert.Nil(t, result)
}

func TestPaginationToV1beta1_WithContinuation(t *testing.T) {
	token := "continuation-token-abc"
	pagination := &model.Pagination{
		Limit:        100,
		Continuation: &token,
	}

	result := paginationToV1beta1(pagination)

	require.NotNil(t, result)
	assert.Equal(t, uint32(100), result.Limit)
	assert.Equal(t, "continuation-token-abc", *result.ContinuationToken)
}

func TestPaginationToV1beta1_WithoutContinuation(t *testing.T) {
	pagination := &model.Pagination{
		Limit:        250,
		Continuation: nil,
	}

	result := paginationToV1beta1(pagination)

	require.NotNil(t, result)
	assert.Equal(t, uint32(250), result.Limit)
	assert.Nil(t, result.ContinuationToken)
}

func TestLookupResourcesCommandToV1beta1_NoPagination(t *testing.T) {
	subject, err := buildTestSubjectReference("user-1")
	require.NoError(t, err)
	resourceType, err := model.NewResourceType("host")
	require.NoError(t, err)
	reporterType, err := model.NewReporterType("hbi")
	require.NoError(t, err)
	relation, err := model.NewRelation("view")
	require.NoError(t, err)

	cmd := LookupResourcesCommand{
		ResourceType: resourceType,
		ReporterType: reporterType,
		Relation:     relation,
		Subject:      subject,
		Pagination:   nil,
		Consistency:  model.NewConsistencyUnspecified(),
	}

	req := lookupResourcesCommandToV1beta1(cmd)

	require.NotNil(t, req)
	assert.Nil(t, req.Pagination)
}

func TestLookupResourcesCommandToV1beta1_WithPagination(t *testing.T) {
	subject, err := buildTestSubjectReference("user-1")
	require.NoError(t, err)
	resourceType, err := model.NewResourceType("host")
	require.NoError(t, err)
	reporterType, err := model.NewReporterType("hbi")
	require.NoError(t, err)
	relation, err := model.NewRelation("view")
	require.NoError(t, err)

	token := "test-token"
	cmd := LookupResourcesCommand{
		ResourceType: resourceType,
		ReporterType: reporterType,
		Relation:     relation,
		Subject:      subject,
		Pagination: &model.Pagination{
			Limit:        150,
			Continuation: &token,
		},
		Consistency: model.NewConsistencyUnspecified(),
	}

	req := lookupResourcesCommandToV1beta1(cmd)

	require.NotNil(t, req)
	require.NotNil(t, req.Pagination)
	assert.Equal(t, uint32(150), req.Pagination.Limit)
	assert.Equal(t, "test-token", *req.Pagination.ContinuationToken)
}

func TestLookupSubjectsCommandToV1beta1_NoPagination(t *testing.T) {
	resourceType, err := model.NewResourceType("host")
	require.NoError(t, err)
	reporterType, err := model.NewReporterType("hbi")
	require.NoError(t, err)
	localResourceId, err := model.NewLocalResourceId("resource-123")
	require.NoError(t, err)
	reporterResourceKey, err := model.NewReporterResourceKey(localResourceId, resourceType, reporterType, model.ReporterInstanceId(""))
	require.NoError(t, err)
	relation, err := model.NewRelation("member")
	require.NoError(t, err)
	subjectType, err := model.NewResourceType("principal")
	require.NoError(t, err)
	subjectReporter, err := model.NewReporterType("rbac")
	require.NoError(t, err)

	cmd := LookupSubjectsCommand{
		Resource:        reporterResourceKey,
		Relation:        relation,
		SubjectType:     subjectType,
		SubjectReporter: subjectReporter,
		SubjectRelation: nil,
		Pagination:      nil,
		Consistency:     model.NewConsistencyUnspecified(),
	}

	req := lookupSubjectsCommandToV1beta1(cmd)

	require.NotNil(t, req)
	assert.Nil(t, req.Pagination)
}

func TestLookupSubjectsCommandToV1beta1_WithPagination(t *testing.T) {
	resourceType, err := model.NewResourceType("host")
	require.NoError(t, err)
	reporterType, err := model.NewReporterType("hbi")
	require.NoError(t, err)
	localResourceId, err := model.NewLocalResourceId("resource-456")
	require.NoError(t, err)
	reporterResourceKey, err := model.NewReporterResourceKey(localResourceId, resourceType, reporterType, model.ReporterInstanceId(""))
	require.NoError(t, err)
	relation, err := model.NewRelation("member")
	require.NoError(t, err)
	subjectType, err := model.NewResourceType("principal")
	require.NoError(t, err)
	subjectReporter, err := model.NewReporterType("rbac")
	require.NoError(t, err)

	cmd := LookupSubjectsCommand{
		Resource:        reporterResourceKey,
		Relation:        relation,
		SubjectType:     subjectType,
		SubjectReporter: subjectReporter,
		SubjectRelation: nil,
		Pagination: &model.Pagination{
			Limit:        200,
			Continuation: nil,
		},
		Consistency: model.NewConsistencyUnspecified(),
	}

	req := lookupSubjectsCommandToV1beta1(cmd)

	require.NotNil(t, req)
	require.NotNil(t, req.Pagination)
	assert.Equal(t, uint32(200), req.Pagination.Limit)
	assert.Nil(t, req.Pagination.ContinuationToken)
}
