package tuples

import (
	"context"
	"io"

	"github.com/go-kratos/kratos/v2/log"
	pb "github.com/project-kessel/inventory-api/api/kessel/inventory/v1beta2"
	"github.com/project-kessel/inventory-api/internal/biz/model"
	relationspb "github.com/project-kessel/relations-api/api/kessel/relations/v1beta1"
)

// TupleService implements the deprecated KesselTupleService.
// This service exists only for RBAC backward compatibility and should not be extended.
// All operations are proxied to relations-api via the Authorizer interface.
type TupleService struct {
	pb.UnimplementedKesselTupleServiceServer
	authz model.Authorizer
}

// New creates a new TupleService with the given authorizer.
func New(authz model.Authorizer) *TupleService {
	return &TupleService{
		authz: authz,
	}
}

// CreateTuples creates relationship tuples.
// DEPRECATED: This endpoint exists only for RBAC backward compatibility.
func (s *TupleService) CreateTuples(ctx context.Context, req *pb.CreateTuplesRequest) (*pb.CreateTuplesResponse, error) {
	log.Warn("DEPRECATED: CreateTuples called - this endpoint is for RBAC-only backward compatibility")

	// Convert inventory-api types to relations-api v1beta1 types
	relReq := &relationspb.CreateTuplesRequest{
		Upsert: req.GetUpsert(),
		Tuples: relationshipsToV1beta1(req.GetTuples()),
	}

	if req.GetFencingCheck() != nil {
		relReq.FencingCheck = fencingCheckToV1beta1(req.GetFencingCheck())
	}

	// Call relations-api via authorizer
	resp, err := s.authz.CreateTuples(ctx, relReq)
	if err != nil {
		return nil, err
	}

	return &pb.CreateTuplesResponse{
		ConsistencyToken: consistencyTokenFromV1beta1(resp.GetConsistencyToken()),
	}, nil
}

// DeleteTuples deletes relationship tuples.
// DEPRECATED: This endpoint exists only for RBAC backward compatibility.
func (s *TupleService) DeleteTuples(ctx context.Context, req *pb.DeleteTuplesRequest) (*pb.DeleteTuplesResponse, error) {
	log.Warn("DEPRECATED: DeleteTuples called - this endpoint is for RBAC-only backward compatibility")

	relReq := &relationspb.DeleteTuplesRequest{
		Filter: tupleFilterToV1beta1(req.GetFilter()),
	}

	if req.GetFencingCheck() != nil {
		relReq.FencingCheck = fencingCheckToV1beta1(req.GetFencingCheck())
	}

	resp, err := s.authz.DeleteTuples(ctx, relReq)
	if err != nil {
		return nil, err
	}

	return &pb.DeleteTuplesResponse{
		ConsistencyToken: consistencyTokenFromV1beta1(resp.GetConsistencyToken()),
	}, nil
}

// ReadTuples reads relationship tuples via server-side streaming.
// DEPRECATED: This endpoint exists only for RBAC backward compatibility.
func (s *TupleService) ReadTuples(req *pb.ReadTuplesRequest, stream pb.KesselTupleService_ReadTuplesServer) error {
	log.Warn("DEPRECATED: ReadTuples called - this endpoint is for RBAC-only backward compatibility")

	ctx := stream.Context()

	// Convert request to relations-api v1beta1 format
	relReq := &relationspb.ReadTuplesRequest{
		Filter: tupleFilterToV1beta1(req.GetFilter()),
	}

	if req.GetPagination() != nil {
		relPagination := &relationspb.RequestPagination{
			Limit: req.GetPagination().GetLimit(),
		}
		if req.GetPagination().ContinuationToken != nil {
			relPagination.ContinuationToken = req.GetPagination().ContinuationToken
		}
		relReq.Pagination = relPagination
	}

	if req.GetConsistency() != nil {
		relReq.Consistency = consistencyToV1beta1(req.GetConsistency())
	}

	// Get stream from relations-api via authorizer
	relStream, err := s.authz.ReadTuples(ctx, relReq)
	if err != nil {
		return err
	}

	// Stream responses, converting each message
	for {
		relResp, err := relStream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		// Convert and send response
		err = stream.Send(&pb.ReadTuplesResponse{
			Tuple:            relationshipFromV1beta1(relResp.GetTuple()),
			Pagination:       &pb.ResponsePagination{ContinuationToken: relResp.GetPagination().GetContinuationToken()},
			ConsistencyToken: consistencyTokenFromV1beta1(relResp.GetConsistencyToken()),
		})
		if err != nil {
			return err
		}
	}

	return nil
}

// AcquireLock acquires a distributed lock.
// DEPRECATED: This endpoint exists only for RBAC backward compatibility.
func (s *TupleService) AcquireLock(ctx context.Context, req *pb.AcquireLockRequest) (*pb.AcquireLockResponse, error) {
	log.Warn("DEPRECATED: AcquireLock called - this endpoint is for RBAC-only backward compatibility")

	relReq := &relationspb.AcquireLockRequest{
		LockId: req.GetLockId(),
	}

	resp, err := s.authz.AcquireLock(ctx, relReq)
	if err != nil {
		return nil, err
	}

	return &pb.AcquireLockResponse{
		LockToken: resp.GetLockToken(),
	}, nil
}

// Conversion functions: inventory-api proto → relations-api v1beta1

// relationshipsToV1beta1 converts an array of inventory relationships to relations-api format.
func relationshipsToV1beta1(rels []*pb.Relationship) []*relationspb.Relationship {
	if rels == nil {
		return nil
	}
	result := make([]*relationspb.Relationship, len(rels))
	for i, rel := range rels {
		result[i] = relationshipToV1beta1(rel)
	}
	return result
}

// relationshipToV1beta1 converts a single inventory relationship to relations-api format.
func relationshipToV1beta1(rel *pb.Relationship) *relationspb.Relationship {
	if rel == nil {
		return nil
	}
	return &relationspb.Relationship{
		Resource: objectReferenceToV1beta1(rel.GetResource()),
		Relation: rel.GetRelation(),
		Subject:  subjectReferenceToV1beta1(rel.GetSubject()),
	}
}

// relationshipFromV1beta1 converts a relations-api relationship back to inventory format.
func relationshipFromV1beta1(rel *relationspb.Relationship) *pb.Relationship {
	if rel == nil {
		return nil
	}
	return &pb.Relationship{
		Resource: objectReferenceFromV1beta1(rel.GetResource()),
		Relation: rel.GetRelation(),
		Subject:  subjectReferenceFromV1beta1(rel.GetSubject()),
	}
}

// objectReferenceToV1beta1 converts inventory object reference to relations-api format.
func objectReferenceToV1beta1(ref *pb.RelationObjectReference) *relationspb.ObjectReference {
	if ref == nil {
		return nil
	}
	return &relationspb.ObjectReference{
		Type: &relationspb.ObjectType{
			Namespace: ref.GetType().GetNamespace(),
			Name:      ref.GetType().GetName(),
		},
		Id: ref.GetId(),
	}
}

// objectReferenceFromV1beta1 converts relations-api object reference back to inventory format.
func objectReferenceFromV1beta1(ref *relationspb.ObjectReference) *pb.RelationObjectReference {
	if ref == nil {
		return nil
	}
	return &pb.RelationObjectReference{
		Type: &pb.RelationObjectType{
			Namespace: ref.GetType().GetNamespace(),
			Name:      ref.GetType().GetName(),
		},
		Id: ref.GetId(),
	}
}

// subjectReferenceToV1beta1 converts inventory subject reference to relations-api format.
func subjectReferenceToV1beta1(ref *pb.RelationSubjectReference) *relationspb.SubjectReference {
	if ref == nil {
		return nil
	}
	result := &relationspb.SubjectReference{
		Subject: objectReferenceToV1beta1(ref.GetSubject()),
	}
	if ref.Relation != nil {
		result.Relation = ref.Relation
	}
	return result
}

// subjectReferenceFromV1beta1 converts relations-api subject reference back to inventory format.
func subjectReferenceFromV1beta1(ref *relationspb.SubjectReference) *pb.RelationSubjectReference {
	if ref == nil {
		return nil
	}
	result := &pb.RelationSubjectReference{
		Subject: objectReferenceFromV1beta1(ref.GetSubject()),
	}
	if ref.Relation != nil {
		result.Relation = ref.Relation
	}
	return result
}

// tupleFilterToV1beta1 converts inventory tuple filter to relations-api format.
func tupleFilterToV1beta1(filter *pb.RelationTupleFilter) *relationspb.RelationTupleFilter {
	if filter == nil {
		return nil
	}
	result := &relationspb.RelationTupleFilter{}

	if filter.ResourceNamespace != nil {
		result.ResourceNamespace = filter.ResourceNamespace
	}
	if filter.ResourceType != nil {
		result.ResourceType = filter.ResourceType
	}
	if filter.ResourceId != nil {
		result.ResourceId = filter.ResourceId
	}
	if filter.Relation != nil {
		result.Relation = filter.Relation
	}
	if filter.SubjectFilter != nil {
		result.SubjectFilter = subjectFilterToV1beta1(filter.SubjectFilter)
	}

	return result
}

// subjectFilterToV1beta1 converts inventory subject filter to relations-api format.
func subjectFilterToV1beta1(filter *pb.RelationSubjectFilter) *relationspb.SubjectFilter {
	if filter == nil {
		return nil
	}
	result := &relationspb.SubjectFilter{}

	if filter.SubjectNamespace != nil {
		result.SubjectNamespace = filter.SubjectNamespace
	}
	if filter.SubjectType != nil {
		result.SubjectType = filter.SubjectType
	}
	if filter.SubjectId != nil {
		result.SubjectId = filter.SubjectId
	}
	if filter.Relation != nil {
		result.Relation = filter.Relation
	}

	return result
}

// fencingCheckToV1beta1 converts inventory fencing check to relations-api format.
func fencingCheckToV1beta1(fc *pb.RelationFencingCheck) *relationspb.FencingCheck {
	if fc == nil {
		return nil
	}
	return &relationspb.FencingCheck{
		LockId:    fc.GetLockId(),
		LockToken: fc.GetLockToken(),
	}
}

// consistencyToV1beta1 converts inventory consistency to relations-api v1beta1 format.
// Note: Inventory has 3 consistency modes, relations-api has 2.
// The at_least_as_acknowledged mode is mapped to minimize_latency as a safe downgrade.
func consistencyToV1beta1(c *pb.Consistency) *relationspb.Consistency {
	if c == nil {
		return nil
	}

	if c.GetMinimizeLatency() {
		return &relationspb.Consistency{
			Requirement: &relationspb.Consistency_MinimizeLatency{MinimizeLatency: true},
		}
	}
	if c.GetAtLeastAsFresh() != nil {
		return &relationspb.Consistency{
			Requirement: &relationspb.Consistency_AtLeastAsFresh{
				AtLeastAsFresh: &relationspb.ConsistencyToken{
					Token: c.GetAtLeastAsFresh().GetToken(),
				},
			},
		}
	}

	// at_least_as_acknowledged not supported in relations-api - default to minimize_latency
	log.Debug("Mapping at_least_as_acknowledged to minimize_latency (relations-api doesn't support this mode)")
	return &relationspb.Consistency{
		Requirement: &relationspb.Consistency_MinimizeLatency{MinimizeLatency: true},
	}
}

// consistencyTokenFromV1beta1 converts relations-api v1beta1 consistency token back to inventory format.
func consistencyTokenFromV1beta1(token *relationspb.ConsistencyToken) *pb.ConsistencyToken {
	if token == nil {
		return nil
	}
	return &pb.ConsistencyToken{Token: token.GetToken()}
}
