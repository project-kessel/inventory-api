package resources_test

import (
	"testing"

	pb "github.com/project-kessel/inventory-api/api/kessel/inventory/v1beta2"
	svc "github.com/project-kessel/inventory-api/internal/service/resources"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestToLookupResourcesCommand_WithLimitOnly(t *testing.T) {
	permission := "view"
	reporterType := "hbi"
	input := &pb.StreamedListObjectsRequest{
		ObjectType: &pb.RepresentationType{
			ResourceType: "host",
			ReporterType: &reporterType,
		},
		Relation: "view",
		Subject: &pb.SubjectReference{
			Relation: &permission,
			Resource: &pb.ResourceReference{
				ResourceId:   "res-id",
				ResourceType: "principal",
				Reporter: &pb.ReporterReference{
					Type: "rbac",
				},
			},
		},
		Pagination: &pb.RequestPagination{
			Limit: 50,
			// ContinuationToken not specified
		},
	}

	result, err := svc.ToLookupResourcesCommand(input)
	require.NoError(t, err)

	require.NotNil(t, result.Pagination)
	assert.Equal(t, uint32(50), result.Pagination.Limit)
	assert.Nil(t, result.Pagination.Continuation)
}

func TestToLookupResourcesCommand_WithContinuationOnly(t *testing.T) {
	// This request has continuation but no limit (defaults to 0).
	// Proto validation should reject this (limit must be > 0), but our converter
	// just passes it through - validation happens before we reach this code.
	// This test verifies the pure converter behavior.
	permission := "view"
	reporterType := "hbi"
	token := "continuation-token-123"
	input := &pb.StreamedListObjectsRequest{
		ObjectType: &pb.RepresentationType{
			ResourceType: "host",
			ReporterType: &reporterType,
		},
		Relation: "view",
		Subject: &pb.SubjectReference{
			Relation: &permission,
			Resource: &pb.ResourceReference{
				ResourceId:   "res-id",
				ResourceType: "principal",
				Reporter: &pb.ReporterReference{
					Type: "rbac",
				},
			},
		},
		Pagination: &pb.RequestPagination{
			// Limit not specified (defaults to 0 in proto3)
			ContinuationToken: &token,
		},
	}

	result, err := svc.ToLookupResourcesCommand(input)
	require.NoError(t, err)

	require.NotNil(t, result.Pagination)
	assert.Equal(t, uint32(0), result.Pagination.Limit, "Pure converter passes through limit=0")
	assert.NotNil(t, result.Pagination.Continuation)
	assert.Equal(t, "continuation-token-123", *result.Pagination.Continuation)
}

func TestToLookupResourcesCommand_WithBothLimitAndContinuation(t *testing.T) {
	permission := "view"
	reporterType := "hbi"
	token := "continuation-token-456"
	input := &pb.StreamedListObjectsRequest{
		ObjectType: &pb.RepresentationType{
			ResourceType: "host",
			ReporterType: &reporterType,
		},
		Relation: "view",
		Subject: &pb.SubjectReference{
			Relation: &permission,
			Resource: &pb.ResourceReference{
				ResourceId:   "res-id",
				ResourceType: "principal",
				Reporter: &pb.ReporterReference{
					Type: "rbac",
				},
			},
		},
		Pagination: &pb.RequestPagination{
			Limit:             100,
			ContinuationToken: &token,
		},
	}

	result, err := svc.ToLookupResourcesCommand(input)
	require.NoError(t, err)

	require.NotNil(t, result.Pagination)
	assert.Equal(t, uint32(100), result.Pagination.Limit)
	assert.NotNil(t, result.Pagination.Continuation)
	assert.Equal(t, "continuation-token-456", *result.Pagination.Continuation)
}

func TestToLookupResourcesCommand_WithEmptyContinuationToken(t *testing.T) {
	permission := "view"
	reporterType := "hbi"
	emptyToken := ""
	input := &pb.StreamedListObjectsRequest{
		ObjectType: &pb.RepresentationType{
			ResourceType: "host",
			ReporterType: &reporterType,
		},
		Relation: "view",
		Subject: &pb.SubjectReference{
			Relation: &permission,
			Resource: &pb.ResourceReference{
				ResourceId:   "res-id",
				ResourceType: "principal",
				Reporter: &pb.ReporterReference{
					Type: "rbac",
				},
			},
		},
		Pagination: &pb.RequestPagination{
			Limit:             50,
			ContinuationToken: &emptyToken,
		},
	}

	result, err := svc.ToLookupResourcesCommand(input)
	require.NoError(t, err)

	require.NotNil(t, result.Pagination)
	assert.Equal(t, uint32(50), result.Pagination.Limit)
	assert.NotNil(t, result.Pagination.Continuation, "Pure converter passes through empty string")
	assert.Equal(t, "", *result.Pagination.Continuation)
}

func TestToLookupSubjectsCommand_WithLimitOnly(t *testing.T) {
	reporterType := "hbi"
	input := &pb.StreamedListSubjectsRequest{
		Resource: &pb.ResourceReference{
			ResourceId:   "resource-1",
			ResourceType: "host",
			Reporter: &pb.ReporterReference{
				Type: reporterType,
			},
		},
		Relation: "view",
		SubjectType: &pb.RepresentationType{
			ResourceType: "principal",
			ReporterType: &reporterType,
		},
		Pagination: &pb.RequestPagination{
			Limit: 75,
			// ContinuationToken not specified
		},
	}

	result, err := svc.ToLookupSubjectsCommand(input)
	require.NoError(t, err)

	require.NotNil(t, result.Pagination)
	assert.Equal(t, uint32(75), result.Pagination.Limit)
	assert.Nil(t, result.Pagination.Continuation)
}

func TestToLookupSubjectsCommand_WithContinuationOnly(t *testing.T) {
	// This request has continuation but no limit (defaults to 0).
	// Proto validation should reject this (limit must be > 0), but our converter
	// just passes it through - validation happens before we reach this code.
	// NOTE: Continuation tokens don't work for LookupSubjects in SpiceDB anyway.
	reporterType := "hbi"
	token := "subjects-token-789"
	input := &pb.StreamedListSubjectsRequest{
		Resource: &pb.ResourceReference{
			ResourceId:   "resource-1",
			ResourceType: "host",
			Reporter: &pb.ReporterReference{
				Type: reporterType,
			},
		},
		Relation: "view",
		SubjectType: &pb.RepresentationType{
			ResourceType: "principal",
			ReporterType: &reporterType,
		},
		Pagination: &pb.RequestPagination{
			// Limit not specified (defaults to 0 in proto3)
			ContinuationToken: &token,
		},
	}

	result, err := svc.ToLookupSubjectsCommand(input)
	require.NoError(t, err)

	require.NotNil(t, result.Pagination)
	assert.Equal(t, uint32(0), result.Pagination.Limit, "Pure converter passes through limit=0")
	assert.NotNil(t, result.Pagination.Continuation)
	assert.Equal(t, "subjects-token-789", *result.Pagination.Continuation)
}

func TestToLookupSubjectsCommand_WithBothLimitAndContinuation(t *testing.T) {
	reporterType := "hbi"
	token := "subjects-token-999"
	input := &pb.StreamedListSubjectsRequest{
		Resource: &pb.ResourceReference{
			ResourceId:   "resource-1",
			ResourceType: "host",
			Reporter: &pb.ReporterReference{
				Type: reporterType,
			},
		},
		Relation: "view",
		SubjectType: &pb.RepresentationType{
			ResourceType: "principal",
			ReporterType: &reporterType,
		},
		Pagination: &pb.RequestPagination{
			Limit:             200,
			ContinuationToken: &token,
		},
	}

	result, err := svc.ToLookupSubjectsCommand(input)
	require.NoError(t, err)

	require.NotNil(t, result.Pagination)
	assert.Equal(t, uint32(200), result.Pagination.Limit)
	assert.NotNil(t, result.Pagination.Continuation)
	assert.Equal(t, "subjects-token-999", *result.Pagination.Continuation)
}

func TestPaginationFromProto_NilInput(t *testing.T) {
	// This test verifies behavior when proto Pagination is completely absent
	reporterType := "hbi"
	input := &pb.StreamedListObjectsRequest{
		ObjectType: &pb.RepresentationType{
			ResourceType: "host",
			ReporterType: &reporterType,
		},
		Relation: "view",
		Subject: &pb.SubjectReference{
			Resource: &pb.ResourceReference{
				ResourceId:   "res-id",
				ResourceType: "principal",
				Reporter: &pb.ReporterReference{
					Type: "rbac",
				},
			},
		},
		// Pagination field is nil
	}

	result, err := svc.ToLookupResourcesCommand(input)
	require.NoError(t, err)

	// When pagination is nil from proto, the Pagination pointer should be nil
	assert.Nil(t, result.Pagination)
}
