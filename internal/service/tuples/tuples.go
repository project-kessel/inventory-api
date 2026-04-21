package tuples

import (
	"context"
	"fmt"
	"io"

	pb "github.com/project-kessel/inventory-api/api/kessel/inventory/v1beta2"
	"github.com/project-kessel/inventory-api/internal/biz/model"
	tuplesctl "github.com/project-kessel/inventory-api/internal/biz/usecase/tuples"
)

// TupleService implements the deprecated KesselTupleService.
// This service exists only for RBAC backward compatibility and should not be extended.
type TupleService struct {
	pb.UnimplementedKesselTupleServiceServer
	Ctl *tuplesctl.TupleCrudUseCase
}

// New creates a new TupleService with the given usecase.
func New(uc *tuplesctl.TupleCrudUseCase) *TupleService {
	return &TupleService{Ctl: uc}
}

// CreateTuples creates relationship tuples (DEPRECATED).
// This endpoint exists only for RBAC backward compatibility.
func (s *TupleService) CreateTuples(ctx context.Context, req *pb.CreateTuplesRequest) (*pb.CreateTuplesResponse, error) {
	cmd, err := toCreateTuplesCommand(req)
	if err != nil {
		return nil, err
	}

	result, err := s.Ctl.CreateTuples(ctx, cmd)
	if err != nil {
		return nil, err
	}

	return fromCreateTuplesResult(result), nil
}

// DeleteTuples deletes relationship tuples (DEPRECATED).
// This endpoint exists only for RBAC backward compatibility.
func (s *TupleService) DeleteTuples(ctx context.Context, req *pb.DeleteTuplesRequest) (*pb.DeleteTuplesResponse, error) {
	cmd, err := toDeleteTuplesCommand(req)
	if err != nil {
		return nil, err
	}

	result, err := s.Ctl.DeleteTuples(ctx, cmd)
	if err != nil {
		return nil, err
	}

	return fromDeleteTuplesResult(result), nil
}

// ReadTuples reads relationship tuples via server-side streaming (DEPRECATED).
// This endpoint exists only for RBAC backward compatibility.
func (s *TupleService) ReadTuples(req *pb.ReadTuplesRequest, stream pb.KesselTupleService_ReadTuplesServer) error {
	ctx := stream.Context()

	cmd, err := toReadTuplesCommand(req)
	if err != nil {
		return err
	}

	modelStream, err := s.Ctl.ReadTuples(ctx, cmd)
	if err != nil {
		return err
	}

	for {
		item, err := modelStream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		resp := readTuplesItemToProto(item)
		if err := stream.Send(resp); err != nil {
			return err
		}
	}

	return nil
}

// AcquireLock acquires a distributed lock (DEPRECATED).
// This endpoint exists only for RBAC backward compatibility.
func (s *TupleService) AcquireLock(ctx context.Context, req *pb.AcquireLockRequest) (*pb.AcquireLockResponse, error) {
	cmd := toAcquireLockCommand(req)

	result, err := s.Ctl.AcquireLock(ctx, cmd)
	if err != nil {
		return nil, err
	}

	return &pb.AcquireLockResponse{
		LockToken: result.LockToken,
	}, nil
}

// --- proto → domain command converters ---

func toCreateTuplesCommand(req *pb.CreateTuplesRequest) (tuplesctl.CreateTuplesCommand, error) {
	tuples, err := relationshipsToRelationsTuples(req.GetTuples())
	if err != nil {
		return tuplesctl.CreateTuplesCommand{}, err
	}

	cmd := tuplesctl.CreateTuplesCommand{
		Tuples: tuples,
		Upsert: req.GetUpsert(),
	}

	if req.GetFencingCheck() != nil {
		cmd.FencingCheck = &tuplesctl.FencingCheck{
			LockId:    req.GetFencingCheck().GetLockId(),
			LockToken: req.GetFencingCheck().GetLockToken(),
		}
	}

	return cmd, nil
}

func toDeleteTuplesCommand(req *pb.DeleteTuplesRequest) (tuplesctl.DeleteTuplesCommand, error) {
	cmd := tuplesctl.DeleteTuplesCommand{
		Filter: tupleFilterFromProto(req.GetFilter()),
	}

	if req.GetFencingCheck() != nil {
		cmd.FencingCheck = &tuplesctl.FencingCheck{
			LockId:    req.GetFencingCheck().GetLockId(),
			LockToken: req.GetFencingCheck().GetLockToken(),
		}
	}

	return cmd, nil
}

func toReadTuplesCommand(req *pb.ReadTuplesRequest) (tuplesctl.ReadTuplesCommand, error) {
	return tuplesctl.ReadTuplesCommand{
		Filter:      tupleFilterFromProto(req.GetFilter()),
		Pagination:  paginationFromProto(req.GetPagination()),
		Consistency: consistencyFromProto(req.GetConsistency()),
	}, nil
}

func toAcquireLockCommand(req *pb.AcquireLockRequest) tuplesctl.AcquireLockCommand {
	return tuplesctl.AcquireLockCommand{
		LockId: req.GetLockId(),
	}
}

// --- proto → domain helpers ---

func relationshipsToRelationsTuples(rels []*pb.Relationship) ([]model.RelationsTuple, error) {
	tuples := make([]model.RelationsTuple, len(rels))
	for i, rel := range rels {
		tuple, err := relationshipToRelationsTuple(rel)
		if err != nil {
			return nil, fmt.Errorf("invalid relationship at index %d: %w", i, err)
		}
		tuples[i] = tuple
	}
	return tuples, nil
}

func relationshipToRelationsTuple(rel *pb.Relationship) (model.RelationsTuple, error) {
	resourceId, err := model.NewLocalResourceId(rel.GetResource().GetId())
	if err != nil {
		return model.RelationsTuple{}, fmt.Errorf("invalid resource ID: %w", err)
	}
	resourceType := model.NewRelationsObjectType(
		rel.GetResource().GetType().GetName(),
		rel.GetResource().GetType().GetNamespace(),
	)
	resource := model.NewRelationsResource(resourceId, resourceType)

	subjectId, err := model.NewLocalResourceId(rel.GetSubject().GetSubject().GetId())
	if err != nil {
		return model.RelationsTuple{}, fmt.Errorf("invalid subject ID: %w", err)
	}
	subjectType := model.NewRelationsObjectType(
		rel.GetSubject().GetSubject().GetType().GetName(),
		rel.GetSubject().GetSubject().GetType().GetNamespace(),
	)
	subjectResource := model.NewRelationsResource(subjectId, subjectType)

	var subjectRelation *model.Relation
	if rel.GetSubject().Relation != nil {
		r := model.DeserializeRelation(*rel.GetSubject().Relation)
		subjectRelation = &r
	}
	subject := model.NewRelationsSubject(subjectResource, subjectRelation)

	return model.NewRelationsTuple(resource, model.DeserializeRelation(rel.GetRelation()), subject), nil
}

func tupleFilterFromProto(pf *pb.RelationTupleFilter) model.TupleFilter {
	if pf == nil {
		return model.TupleFilter{}
	}

	filter := model.TupleFilter{
		ResourceNamespace: pf.ResourceNamespace,
		ResourceType:      pf.ResourceType,
		ResourceId:        pf.ResourceId,
		Relation:          pf.Relation,
	}

	if pf.GetSubjectFilter() != nil {
		filter.SubjectFilter = &model.TupleSubjectFilter{
			SubjectNamespace: pf.GetSubjectFilter().SubjectNamespace,
			SubjectType:      pf.GetSubjectFilter().SubjectType,
			SubjectId:        pf.GetSubjectFilter().SubjectId,
			Relation:         pf.GetSubjectFilter().Relation,
		}
	}

	return filter
}

func paginationFromProto(p *pb.RequestPagination) *model.Pagination {
	if p == nil {
		return nil
	}
	return &model.Pagination{
		Limit:        p.Limit,
		Continuation: p.ContinuationToken,
	}
}

func consistencyFromProto(c *pb.Consistency) model.Consistency {
	if c == nil {
		return model.NewConsistencyUnspecified()
	}
	if c.GetMinimizeLatency() {
		return model.NewConsistencyMinimizeLatency()
	}
	if c.GetAtLeastAsAcknowledged() {
		return model.NewConsistencyAtLeastAsAcknowledged()
	}
	if c.GetAtLeastAsFresh() != nil {
		token := model.DeserializeConsistencyToken(c.GetAtLeastAsFresh().GetToken())
		return model.NewConsistencyAtLeastAsFresh(token)
	}
	return model.NewConsistencyUnspecified()
}

// --- domain → proto converters ---

func fromCreateTuplesResult(result *tuplesctl.CreateTuplesResult) *pb.CreateTuplesResponse {
	if result.ConsistencyToken == "" {
		return &pb.CreateTuplesResponse{}
	}
	return &pb.CreateTuplesResponse{
		ConsistencyToken: &pb.ConsistencyToken{Token: result.ConsistencyToken.Serialize()},
	}
}

func fromDeleteTuplesResult(result *tuplesctl.DeleteTuplesResult) *pb.DeleteTuplesResponse {
	if result.ConsistencyToken == "" {
		return &pb.DeleteTuplesResponse{}
	}
	return &pb.DeleteTuplesResponse{
		ConsistencyToken: &pb.ConsistencyToken{Token: result.ConsistencyToken.Serialize()},
	}
}

func readTuplesItemToProto(item model.ReadTuplesItem) *pb.ReadTuplesResponse {
	tuple := &pb.Relationship{
		Resource: &pb.RelationObjectReference{
			Type: &pb.RelationObjectType{
				Namespace: item.ResourceNamespace,
				Name:      item.ResourceType,
			},
			Id: item.ResourceId,
		},
		Relation: item.Relation,
		Subject: &pb.RelationSubjectReference{
			Subject: &pb.RelationObjectReference{
				Type: &pb.RelationObjectType{
					Namespace: item.SubjectNamespace,
					Name:      item.SubjectType,
				},
				Id: item.SubjectId,
			},
		},
	}
	if item.SubjectRelation != nil {
		tuple.Subject.Relation = item.SubjectRelation
	}

	resp := &pb.ReadTuplesResponse{
		Tuple: tuple,
	}

	if item.ContinuationToken != "" {
		resp.Pagination = &pb.ResponsePagination{ContinuationToken: item.ContinuationToken}
	}

	if item.ConsistencyToken != "" {
		resp.ConsistencyToken = &pb.ConsistencyToken{Token: item.ConsistencyToken.Serialize()}
	}

	return resp
}
