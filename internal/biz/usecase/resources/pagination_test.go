package resources

import (
	"testing"

	"github.com/project-kessel/inventory-api/internal/biz/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPaginationToV1beta1_BothNil(t *testing.T) {
	pagination := model.Pagination{
		Limit:        nil,
		Continuation: nil,
	}

	result := paginationToV1beta1(pagination)

	assert.Nil(t, result, "Should return nil when no pagination fields are specified")
}

func TestPaginationToV1beta1_OnlyLimit(t *testing.T) {
	limit := uint32(100)
	pagination := model.Pagination{
		Limit:        &limit,
		Continuation: nil,
	}

	result := paginationToV1beta1(pagination)

	require.NotNil(t, result, "Should create Pagination when limit is specified")
	assert.Equal(t, uint32(100), result.Limit)
	assert.Nil(t, result.ContinuationToken, "ContinuationToken should be nil when not specified")
}

func TestPaginationToV1beta1_OnlyContinuation(t *testing.T) {
	// This tests the "pure converter" behavior: we convert what we're given,
	// even if it will fail proto validation downstream.
	// Proto validation requires limit > 0, so this RequestPagination with limit=0
	// will be REJECTED by relations-api with a validation error.
	token := "continuation-token-abc"
	pagination := model.Pagination{
		Limit:        nil,
		Continuation: &token,
	}

	result := paginationToV1beta1(pagination)

	require.NotNil(t, result, "Converter creates Pagination even though it will fail validation")
	assert.Equal(t, uint32(0), result.Limit, "Limit defaults to 0 when not specified - will FAIL proto validation")
	assert.NotNil(t, result.ContinuationToken)
	assert.Equal(t, "continuation-token-abc", *result.ContinuationToken)
}

func TestPaginationToV1beta1_BothSet(t *testing.T) {
	limit := uint32(250)
	token := "continuation-token-xyz"
	pagination := model.Pagination{
		Limit:        &limit,
		Continuation: &token,
	}

	result := paginationToV1beta1(pagination)

	require.NotNil(t, result, "Should create Pagination when both fields are specified")
	assert.Equal(t, uint32(250), result.Limit)
	assert.NotNil(t, result.ContinuationToken)
	assert.Equal(t, "continuation-token-xyz", *result.ContinuationToken)
}

func TestPaginationToV1beta1_ZeroLimit(t *testing.T) {
	// Edge case: What if user explicitly sets limit to 0?
	// This shouldn't happen in practice due to proto validation,
	// but our converter should handle it correctly
	limit := uint32(0)
	pagination := model.Pagination{
		Limit:        &limit,
		Continuation: nil,
	}

	result := paginationToV1beta1(pagination)

	// Even though limit is 0, it was explicitly set, so we create the object
	require.NotNil(t, result, "Should create Pagination even with zero limit if explicitly set")
	assert.Equal(t, uint32(0), result.Limit)
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
		Pagination: model.Pagination{
			Limit:        nil,
			Continuation: nil,
		},
		Consistency: model.NewConsistencyUnspecified(),
	}

	req := lookupResourcesCommandToV1beta1(cmd)

	require.NotNil(t, req)
	assert.Nil(t, req.Pagination, "Pagination should be nil when no fields are specified")
}

func TestLookupResourcesCommandToV1beta1_WithOnlyLimit(t *testing.T) {
	subject, err := buildTestSubjectReference("user-1")
	require.NoError(t, err)
	resourceType, err := model.NewResourceType("host")
	require.NoError(t, err)
	reporterType, err := model.NewReporterType("hbi")
	require.NoError(t, err)
	relation, err := model.NewRelation("view")
	require.NoError(t, err)

	limit := uint32(150)
	cmd := LookupResourcesCommand{
		ResourceType: resourceType,
		ReporterType: reporterType,
		Relation:     relation,
		Subject:      subject,
		Pagination: model.Pagination{
			Limit:        &limit,
			Continuation: nil,
		},
		Consistency: model.NewConsistencyUnspecified(),
	}

	req := lookupResourcesCommandToV1beta1(cmd)

	require.NotNil(t, req)
	require.NotNil(t, req.Pagination, "Pagination should be present when limit is specified")
	assert.Equal(t, uint32(150), req.Pagination.Limit)
	assert.Nil(t, req.Pagination.ContinuationToken)
}

func TestLookupResourcesCommandToV1beta1_WithOnlyContinuation(t *testing.T) {
	// This tests pure conversion behavior. The converter creates what it's given,
	// but this will FAIL proto validation at relations-api because limit=0 violates
	// the constraint: [(buf.validate.field).uint32 = {gt: 0}]
	// In practice, users should not send continuation-only pagination.
	subject, err := buildTestSubjectReference("user-1")
	require.NoError(t, err)
	resourceType, err := model.NewResourceType("host")
	require.NoError(t, err)
	reporterType, err := model.NewReporterType("hbi")
	require.NoError(t, err)
	relation, err := model.NewRelation("view")
	require.NoError(t, err)

	token := "test-continuation-token"
	cmd := LookupResourcesCommand{
		ResourceType: resourceType,
		ReporterType: reporterType,
		Relation:     relation,
		Subject:      subject,
		Pagination: model.Pagination{
			Limit:        nil,
			Continuation: &token,
		},
		Consistency: model.NewConsistencyUnspecified(),
	}

	req := lookupResourcesCommandToV1beta1(cmd)

	require.NotNil(t, req)
	require.NotNil(t, req.Pagination, "Converter creates Pagination even though it will fail validation")
	assert.Equal(t, uint32(0), req.Pagination.Limit, "Limit defaults to 0 - will FAIL relations-api proto validation")
	assert.NotNil(t, req.Pagination.ContinuationToken)
	assert.Equal(t, "test-continuation-token", *req.Pagination.ContinuationToken)
}

func TestLookupResourcesCommandToV1beta1_WithBothLimitAndContinuation(t *testing.T) {
	subject, err := buildTestSubjectReference("user-1")
	require.NoError(t, err)
	resourceType, err := model.NewResourceType("host")
	require.NoError(t, err)
	reporterType, err := model.NewReporterType("hbi")
	require.NoError(t, err)
	relation, err := model.NewRelation("view")
	require.NoError(t, err)

	limit := uint32(500)
	token := "both-fields-token"
	cmd := LookupResourcesCommand{
		ResourceType: resourceType,
		ReporterType: reporterType,
		Relation:     relation,
		Subject:      subject,
		Pagination: model.Pagination{
			Limit:        &limit,
			Continuation: &token,
		},
		Consistency: model.NewConsistencyUnspecified(),
	}

	req := lookupResourcesCommandToV1beta1(cmd)

	require.NotNil(t, req)
	require.NotNil(t, req.Pagination, "Pagination should be present when both fields are specified")
	assert.Equal(t, uint32(500), req.Pagination.Limit)
	assert.NotNil(t, req.Pagination.ContinuationToken)
	assert.Equal(t, "both-fields-token", *req.Pagination.ContinuationToken)
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
		Pagination: model.Pagination{
			Limit:        nil,
			Continuation: nil,
		},
		Consistency: model.NewConsistencyUnspecified(),
	}

	req := lookupSubjectsCommandToV1beta1(cmd)

	require.NotNil(t, req)
	assert.Nil(t, req.Pagination, "Pagination should be nil when no fields are specified")
}

func TestLookupSubjectsCommandToV1beta1_WithOnlyContinuation(t *testing.T) {
	// This tests pure conversion behavior. The converter creates what it's given,
	// but this will FAIL proto validation at relations-api because limit=0 violates
	// the constraint: [(buf.validate.field).uint32 = {gt: 0}]
	// NOTE: Continuation tokens don't work for LookupSubjects in SpiceDB anyway,
	// so this scenario is doubly invalid in practice.
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

	token := "subjects-continuation-xyz"
	cmd := LookupSubjectsCommand{
		Resource:        reporterResourceKey,
		Relation:        relation,
		SubjectType:     subjectType,
		SubjectReporter: subjectReporter,
		SubjectRelation: nil,
		Pagination: model.Pagination{
			Limit:        nil,
			Continuation: &token,
		},
		Consistency: model.NewConsistencyUnspecified(),
	}

	req := lookupSubjectsCommandToV1beta1(cmd)

	require.NotNil(t, req)
	require.NotNil(t, req.Pagination, "Converter creates Pagination even though it will fail validation")
	assert.Equal(t, uint32(0), req.Pagination.Limit, "Limit defaults to 0 - will FAIL relations-api proto validation")
	assert.NotNil(t, req.Pagination.ContinuationToken)
	assert.Equal(t, "subjects-continuation-xyz", *req.Pagination.ContinuationToken)
}
