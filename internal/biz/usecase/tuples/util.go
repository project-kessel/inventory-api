package tuples

import (
	"github.com/project-kessel/inventory-api/internal/biz/model"
	relationspb "github.com/project-kessel/relations-api/api/kessel/relations/v1beta1"
)

// createTuplesCommandToV1beta1 converts CreateTuplesCommand to relations-api v1beta1 request.
func createTuplesCommandToV1beta1(cmd CreateTuplesCommand) *relationspb.CreateTuplesRequest {
	req := &relationspb.CreateTuplesRequest{
		Upsert: cmd.Upsert,
		Tuples: relationsTuplesToV1beta1(cmd.Tuples),
	}
	if cmd.FencingCheck != nil {
		req.FencingCheck = fencingCheckToV1beta1(cmd.FencingCheck)
	}
	return req
}

// deleteTuplesCommandToV1beta1 converts DeleteTuplesCommand to relations-api v1beta1 request.
func deleteTuplesCommandToV1beta1(cmd DeleteTuplesCommand) *relationspb.DeleteTuplesRequest {
	req := &relationspb.DeleteTuplesRequest{
		Filter: tupleFilterToV1beta1(cmd.Filter),
	}
	if cmd.FencingCheck != nil {
		req.FencingCheck = fencingCheckToV1beta1(cmd.FencingCheck)
	}
	return req
}

// readTuplesCommandToV1beta1 converts ReadTuplesCommand to relations-api v1beta1 request.
func readTuplesCommandToV1beta1(cmd ReadTuplesCommand) *relationspb.ReadTuplesRequest {
	req := &relationspb.ReadTuplesRequest{
		Filter:      tupleFilterToV1beta1(cmd.Filter),
		Pagination:  paginationToV1beta1(cmd.Pagination),
		Consistency: consistencyToV1beta1(cmd.Consistency),
	}
	return req
}

// acquireLockCommandToV1beta1 converts AcquireLockCommand to relations-api v1beta1 request.
func acquireLockCommandToV1beta1(cmd AcquireLockCommand) *relationspb.AcquireLockRequest {
	return &relationspb.AcquireLockRequest{
		LockId: cmd.LockId,
	}
}

// Helper converters (domain types → v1beta1)

// relationsTuplesToV1beta1 converts an array of domain tuples to v1beta1 relationships.
func relationsTuplesToV1beta1(tuples []model.RelationsTuple) []*relationspb.Relationship {
	result := make([]*relationspb.Relationship, len(tuples))
	for i, tuple := range tuples {
		result[i] = relationsTupleToV1beta1(tuple)
	}
	return result
}

// relationsTupleToV1beta1 converts a single domain tuple to v1beta1 relationship.
func relationsTupleToV1beta1(tuple model.RelationsTuple) *relationspb.Relationship {
	rel := &relationspb.Relationship{
		Resource: &relationspb.ObjectReference{
			Type: &relationspb.ObjectType{
				Namespace: tuple.Resource().Type().Namespace(),
				Name:      tuple.Resource().Type().Name(),
			},
			Id: tuple.Resource().Id().Serialize(),
		},
		Relation: tuple.Relation(),
		Subject: &relationspb.SubjectReference{
			Subject: &relationspb.ObjectReference{
				Type: &relationspb.ObjectType{
					Namespace: tuple.Subject().Subject().Type().Namespace(),
					Name:      tuple.Subject().Subject().Type().Name(),
				},
				Id: tuple.Subject().Subject().Id().Serialize(),
			},
		},
	}

	// Set relation only if non-empty (empty string indicates no relation)
	if tuple.Subject().Relation() != "" {
		subjectRelation := tuple.Subject().Relation()
		rel.Subject.Relation = &subjectRelation
	}

	return rel
}

// tupleFilterToV1beta1 converts domain tuple filter to v1beta1 format.
func tupleFilterToV1beta1(filter TupleFilter) *relationspb.RelationTupleFilter {
	result := &relationspb.RelationTupleFilter{
		ResourceNamespace: filter.ResourceNamespace,
		ResourceType:      filter.ResourceType,
		ResourceId:        filter.ResourceId,
		Relation:          filter.Relation,
	}

	if filter.SubjectFilter != nil {
		result.SubjectFilter = subjectFilterToV1beta1(filter.SubjectFilter)
	}

	return result
}

// subjectFilterToV1beta1 converts domain subject filter to v1beta1 format.
func subjectFilterToV1beta1(filter *SubjectFilter) *relationspb.SubjectFilter {
	if filter == nil {
		return nil
	}
	return &relationspb.SubjectFilter{
		SubjectNamespace: filter.SubjectNamespace,
		SubjectType:      filter.SubjectType,
		SubjectId:        filter.SubjectId,
		Relation:         filter.Relation,
	}
}

// fencingCheckToV1beta1 converts domain fencing check to v1beta1 format.
func fencingCheckToV1beta1(fc *FencingCheck) *relationspb.FencingCheck {
	if fc == nil {
		return nil
	}
	return &relationspb.FencingCheck{
		LockId:    fc.LockId,
		LockToken: fc.LockToken,
	}
}

// paginationToV1beta1 converts model.Pagination to v1beta1 RequestPagination.
// Returns nil if pagination is not specified.
func paginationToV1beta1(pagination *model.Pagination) *relationspb.RequestPagination {
	if pagination == nil {
		return nil
	}
	req := &relationspb.RequestPagination{
		Limit: pagination.Limit,
	}
	if pagination.Continuation != nil {
		req.ContinuationToken = pagination.Continuation
	}
	return req
}

// consistencyToV1beta1 converts model.Consistency to v1beta1 Consistency.
// Note: Inventory has 3 consistency modes, relations-api has 2.
// The at_least_as_acknowledged mode is mapped to minimize_latency as a safe downgrade.
func consistencyToV1beta1(c model.Consistency) *relationspb.Consistency {
	switch model.ConsistencyTypeOf(c) {
	case model.ConsistencyMinimizeLatency:
		return &relationspb.Consistency{
			Requirement: &relationspb.Consistency_MinimizeLatency{MinimizeLatency: true},
		}
	case model.ConsistencyAtLeastAsFresh:
		fresh, _ := model.AsAtLeastAsFresh(c)
		return &relationspb.Consistency{
			Requirement: &relationspb.Consistency_AtLeastAsFresh{
				AtLeastAsFresh: &relationspb.ConsistencyToken{
					Token: fresh.ConsistencyToken().Serialize(),
				},
			},
		}
	case model.ConsistencyAtLeastAsAcknowledged:
		// Relations-api doesn't support at_least_as_acknowledged - default to minimize_latency
		return &relationspb.Consistency{
			Requirement: &relationspb.Consistency_MinimizeLatency{MinimizeLatency: true},
		}
	default:
		// Unspecified - default to minimize_latency
		return &relationspb.Consistency{
			Requirement: &relationspb.Consistency_MinimizeLatency{MinimizeLatency: true},
		}
	}
}
