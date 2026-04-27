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
	objReporterType := model.DeserializeReporterType(rel.GetResource().GetType().GetNamespace())
	objReporter := model.NewReporterReference(objReporterType, nil)
	object := model.NewResourceReference(
		model.DeserializeResourceType(rel.GetResource().GetType().GetName()),
		resourceId,
		&objReporter,
	)

	subjectId, err := model.NewLocalResourceId(rel.GetSubject().GetSubject().GetId())
	if err != nil {
		return model.RelationsTuple{}, fmt.Errorf("invalid subject ID: %w", err)
	}
	subReporterType := model.DeserializeReporterType(rel.GetSubject().GetSubject().GetType().GetNamespace())
	subReporter := model.NewReporterReference(subReporterType, nil)
	subResource := model.NewResourceReference(
		model.DeserializeResourceType(rel.GetSubject().GetSubject().GetType().GetName()),
		subjectId,
		&subReporter,
	)

	var subjectRef model.SubjectReference
	if rel.GetSubject().Relation != nil {
		r := model.DeserializeRelation(*rel.GetSubject().Relation)
		subjectRef = model.NewSubjectReference(subResource, &r)
	} else {
		subjectRef = model.NewSubjectReferenceWithoutRelation(subResource)
	}

	return model.NewRelationsTuple(object, model.DeserializeRelation(rel.GetRelation()), subjectRef), nil
}

func tupleFilterFromProto(pf *pb.RelationTupleFilter) model.TupleFilter {
	if pf == nil {
		return model.NewTupleFilter()
	}

	filter := model.NewTupleFilter()
	if pf.ResourceNamespace != nil {
		filter = filter.WithReporterType(model.DeserializeReporterType(*pf.ResourceNamespace))
	}
	if pf.ResourceType != nil {
		filter = filter.WithObjectType(model.DeserializeResourceType(*pf.ResourceType))
	}
	if pf.ResourceId != nil {
		filter = filter.WithObjectId(model.DeserializeLocalResourceId(*pf.ResourceId))
	}
	if pf.Relation != nil {
		filter = filter.WithRelation(model.DeserializeRelation(*pf.Relation))
	}

	if pf.GetSubjectFilter() != nil {
		sf := model.NewTupleSubjectFilter()
		if pf.GetSubjectFilter().SubjectNamespace != nil {
			sf = sf.WithReporterType(model.DeserializeReporterType(*pf.GetSubjectFilter().SubjectNamespace))
		}
		if pf.GetSubjectFilter().SubjectType != nil {
			sf = sf.WithSubjectType(model.DeserializeResourceType(*pf.GetSubjectFilter().SubjectType))
		}
		if pf.GetSubjectFilter().SubjectId != nil {
			sf = sf.WithSubjectId(model.DeserializeLocalResourceId(*pf.GetSubjectFilter().SubjectId))
		}
		if pf.GetSubjectFilter().Relation != nil {
			sf = sf.WithRelation(model.DeserializeRelation(*pf.GetSubjectFilter().Relation))
		}
		filter = filter.WithSubject(sf)
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
	obj := item.Object()
	sub := item.Subject().Resource()

	objNamespace := ""
	if obj.HasReporter() {
		objNamespace = obj.Reporter().ReporterType().Serialize()
	}
	subNamespace := ""
	if sub.HasReporter() {
		subNamespace = sub.Reporter().ReporterType().Serialize()
	}

	tuple := &pb.Relationship{
		Resource: &pb.RelationObjectReference{
			Type: &pb.RelationObjectType{
				Namespace: objNamespace,
				Name:      obj.ResourceType().Serialize(),
			},
			Id: obj.ResourceId().Serialize(),
		},
		Relation: item.Relation().Serialize(),
		Subject: &pb.RelationSubjectReference{
			Subject: &pb.RelationObjectReference{
				Type: &pb.RelationObjectType{
					Namespace: subNamespace,
					Name:      sub.ResourceType().Serialize(),
				},
				Id: sub.ResourceId().Serialize(),
			},
		},
	}
	if item.Subject().HasRelation() {
		rel := item.Subject().Relation().Serialize()
		tuple.Subject.Relation = &rel
	}

	resp := &pb.ReadTuplesResponse{
		Tuple: tuple,
	}

	if item.ContinuationToken() != "" {
		resp.Pagination = &pb.ResponsePagination{ContinuationToken: item.ContinuationToken()}
	}

	if item.ConsistencyToken() != "" {
		resp.ConsistencyToken = &pb.ConsistencyToken{Token: item.ConsistencyToken().Serialize()}
	}

	return resp
}
