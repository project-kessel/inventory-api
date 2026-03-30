package resources_test

import (
	"testing"

	svc "github.com/project-kessel/inventory-api/internal/service/resources"
	pb "github.com/project-kessel/inventory-api/api/kessel/inventory/v1beta2"
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

	assert.NotNil(t, result.Pagination.Limit)
	assert.Equal(t, uint32(50), *result.Pagination.Limit)
	assert.Nil(t, result.Pagination.Continuation)
}

func TestToLookupResourcesCommand_WithContinuationOnly(t *testing.T) {
	// When user sends continuation-only from inventory-api proto (limit defaults to 0),
	// we treat limit=0 as "not specified" and convert it to nil.
	// Later when converting back to relations-api proto, this will create
	// RequestPagination{Limit: 0} which will FAIL proto validation.
	// Users should not send continuation-only pagination in practice.
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

	assert.Nil(t, result.Pagination.Limit, "Limit=0 from proto treated as 'not specified' (nil)")
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

	assert.NotNil(t, result.Pagination.Limit)
	assert.Equal(t, uint32(100), *result.Pagination.Limit)
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

	assert.NotNil(t, result.Pagination.Limit)
	assert.Equal(t, uint32(50), *result.Pagination.Limit)
	assert.Nil(t, result.Pagination.Continuation, "Empty continuation token should be treated as not specified")
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

	assert.NotNil(t, result.Pagination.Limit)
	assert.Equal(t, uint32(75), *result.Pagination.Limit)
	assert.Nil(t, result.Pagination.Continuation)
}

func TestToLookupSubjectsCommand_WithContinuationOnly(t *testing.T) {
	// When user sends continuation-only from inventory-api proto (limit defaults to 0),
	// we treat limit=0 as "not specified" and convert it to nil.
	// Later when converting back to relations-api proto, this will create
	// RequestPagination{Limit: 0} which will FAIL proto validation.
	// NOTE: Continuation tokens don't work for LookupSubjects in SpiceDB anyway.
	// Users should not send continuation-only pagination in practice.
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

	assert.Nil(t, result.Pagination.Limit, "Limit=0 from proto treated as 'not specified' (nil)")
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

	assert.NotNil(t, result.Pagination.Limit)
	assert.Equal(t, uint32(200), *result.Pagination.Limit)
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

	// When pagination is nil, we should get an empty Pagination struct with nil fields
	assert.Nil(t, result.Pagination.Limit)
	assert.Nil(t, result.Pagination.Continuation)
}
