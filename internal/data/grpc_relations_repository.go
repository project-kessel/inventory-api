package data

import (
	"context"
	"fmt"
	"io"

	"github.com/spf13/viper"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/project-kessel/inventory-api/internal/biz/model"
	"github.com/project-kessel/inventory-api/internal/config/relations/kessel"
	kesselv1 "github.com/project-kessel/relations-api/api/kessel/relations/v1"
	kesselapi "github.com/project-kessel/relations-api/api/kessel/relations/v1beta1"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
)

type GRPCRelationsRepository struct {
	HealthService  kesselv1.KesselRelationsHealthServiceClient
	CheckService   kesselapi.KesselCheckServiceClient
	TupleService   kesselapi.KesselTupleServiceClient
	LookupService  kesselapi.KesselLookupServiceClient
	tokenClient    *tokenClient
	Logger         *log.Helper
	successCounter metric.Int64Counter
	failureCounter metric.Int64Counter
}

var _ model.RelationsRepository = &GRPCRelationsRepository{}

func NewGRPCRelationsRepository(ctx context.Context, config kessel.CompletedConfig, logger *log.Helper) (*GRPCRelationsRepository, error) {
	logger.Info("Using relations repository: kessel")
	tokenCli := NewTokenClient(config.GetTokenConfig())

	meter := otel.Meter("github.com/project-kessel/inventory-api/blob/main/internal/server/otel")

	successCounter, err := meter.Int64Counter("inventory_relations_api_success")
	if err != nil {
		return nil, fmt.Errorf("failed to create success counter: %w", err)
	}

	failureCounter, err := meter.Int64Counter("inventory_relations_api_failure")
	if err != nil {
		return nil, fmt.Errorf("failed to create failure counter: %w", err)
	}

	return &GRPCRelationsRepository{
		HealthService:  kesselv1.NewKesselRelationsHealthServiceClient(config.GetGRPCConn()),
		CheckService:   kesselapi.NewKesselCheckServiceClient(config.GetGRPCConn()),
		TupleService:   kesselapi.NewKesselTupleServiceClient(config.GetGRPCConn()),
		LookupService:  kesselapi.NewKesselLookupServiceClient(config.GetGRPCConn()),
		Logger:         logger,
		tokenClient:    tokenCli,
		successCounter: successCounter,
		failureCounter: failureCounter,
	}, nil
}

func (a *GRPCRelationsRepository) incrFailureCounter(method string) {
	a.failureCounter.Add(context.Background(), 1, metric.WithAttributes(
		attribute.String("method", method),
	))
}

func (a *GRPCRelationsRepository) incrSuccessCounter(method string) {
	a.successCounter.Add(context.Background(), 1, metric.WithAttributes(attribute.String("method", method)))
}

func (a *GRPCRelationsRepository) getCallOptions() ([]grpc.CallOption, error) {
	var opts []grpc.CallOption
	opts = append(opts, grpc.EmptyCallOption{})
	if a.tokenClient.EnableOIDCAuth {
		token, err := a.tokenClient.getToken()
		if err != nil {
			return nil, fmt.Errorf("failed to request token: %w", err)
		}
		if a.tokenClient.Insecure {
			opts = append(opts, WithInsecureBearerToken(token.AccessToken))
		} else {
			opts = append(opts, WithBearerToken(token.AccessToken))
		}
	}
	return opts, nil
}

func (a *GRPCRelationsRepository) Health(ctx context.Context) (model.HealthResult, error) {
	opts, err := a.getCallOptions()
	if err != nil {
		a.incrFailureCounter("Health")
		return model.HealthResult{}, err
	}
	if viper.GetBool("log.readyz") {
		log.Infof("Checking relations-api readyz endpoint")
	}
	resp, err := a.HealthService.GetReadyz(ctx, &kesselv1.GetReadyzRequest{}, opts...)
	if err != nil {
		a.incrFailureCounter("Health")
		return model.HealthResult{}, err
	}

	a.incrSuccessCounter("Health")
	return model.NewHealthResult(resp.GetStatus(), int(resp.GetCode())), nil
}

func (a *GRPCRelationsRepository) Check(ctx context.Context, rel model.Relationship, consistency model.Consistency,
) (bool, model.ConsistencyToken, error) {
	obj := rel.Object()
	log.Infof("Check: on resourceType=%s, localResourceId=%s",
		obj.ResourceType().Serialize(), obj.ResourceId().Serialize())

	opts, err := a.getCallOptions()
	if err != nil {
		a.incrFailureCounter("Check")
		return false, model.MinimizeLatencyToken, err
	}

	resp, err := a.CheckService.Check(ctx, &kesselapi.CheckRequest{
		Resource:    resourceReferenceToV1Beta1(obj),
		Relation:    rel.Relation().Serialize(),
		Subject:     subjectToV1Beta1(rel.Subject()),
		Consistency: consistencyToV1Beta1(consistency),
	}, opts...)

	if err != nil {
		a.incrFailureCounter("Check")
		return false, model.MinimizeLatencyToken, err
	}

	a.incrSuccessCounter("Check")
	return resp.GetAllowed() == kesselapi.CheckResponse_ALLOWED_TRUE,
		tokenFromV1Beta1(resp.GetConsistencyToken()), nil
}

func (a *GRPCRelationsRepository) CheckForUpdate(ctx context.Context, rel model.Relationship,
) (bool, model.ConsistencyToken, error) {
	opts, err := a.getCallOptions()
	if err != nil {
		a.incrFailureCounter("CheckForUpdate")
		return false, model.MinimizeLatencyToken, err
	}

	resp, err := a.CheckService.CheckForUpdate(ctx, &kesselapi.CheckForUpdateRequest{
		Resource: resourceReferenceToV1Beta1(rel.Object()),
		Relation: rel.Relation().Serialize(),
		Subject:  subjectToV1Beta1(rel.Subject()),
	}, opts...)

	if err != nil {
		a.incrFailureCounter("CheckForUpdate")
		return false, model.MinimizeLatencyToken, err
	}

	a.incrSuccessCounter("CheckForUpdate")
	return resp.GetAllowed() == kesselapi.CheckForUpdateResponse_ALLOWED_TRUE,
		tokenFromV1Beta1(resp.GetConsistencyToken()), nil
}

func (a *GRPCRelationsRepository) CheckBulk(ctx context.Context, rels []model.Relationship, consistency model.Consistency,
) (model.CheckBulkResult, error) {
	log.Infof("CheckBulk: checking %d items", len(rels))

	protoItems := relationshipsToCheckBulkV1Beta1(rels)
	protoConsistency := consistencyToV1Beta1(consistency)

	opts, err := a.getCallOptions()
	if err != nil {
		a.incrFailureCounter("CheckBulk")
		return model.CheckBulkResult{}, err
	}

	resp, err := a.CheckService.CheckBulk(ctx, &kesselapi.CheckBulkRequest{
		Items:       protoItems,
		Consistency: protoConsistency,
	}, opts...)
	if err != nil {
		a.incrFailureCounter("CheckBulk")
		return model.CheckBulkResult{}, err
	}

	a.incrSuccessCounter("CheckBulk")
	return checkBulkResultFromV1Beta1(resp.GetPairs(), rels, resp.GetConsistencyToken())
}

func (a *GRPCRelationsRepository) CheckForUpdateBulk(ctx context.Context, rels []model.Relationship,
) (model.CheckBulkResult, error) {
	log.Infof("CheckForUpdateBulk: checking %d items", len(rels))

	protoItems := relationshipsToCheckBulkV1Beta1(rels)

	opts, err := a.getCallOptions()
	if err != nil {
		a.incrFailureCounter("CheckForUpdateBulk")
		return model.CheckBulkResult{}, err
	}

	resp, err := a.CheckService.CheckForUpdateBulk(ctx, &kesselapi.CheckForUpdateBulkRequest{
		Items: protoItems,
	}, opts...)
	if err != nil {
		a.incrFailureCounter("CheckForUpdateBulk")
		return model.CheckBulkResult{}, err
	}

	a.incrSuccessCounter("CheckForUpdateBulk")
	return checkBulkResultFromV1Beta1(resp.GetPairs(), rels, resp.GetConsistencyToken())
}

func (a *GRPCRelationsRepository) LookupObjects(ctx context.Context,
	objectType model.RepresentationType,
	relation model.Relation, subject model.SubjectReference,
	pagination *model.Pagination, consistency model.Consistency,
) (model.ResultStream[model.LookupObjectsItem], error) {
	opts, err := a.getCallOptions()
	if err != nil {
		a.incrFailureCounter("LookupObjects")
		return nil, err
	}

	reporterType, err := objectType.RequireReporterType()
	if err != nil {
		a.incrFailureCounter("LookupObjects")
		return nil, fmt.Errorf("LookupObjects requires reporter type: %w", err)
	}

	stream, err := a.LookupService.LookupResources(ctx, &kesselapi.LookupResourcesRequest{
		ResourceType: &kesselapi.ObjectType{
			Namespace: reporterType.Serialize(),
			Name:      objectType.ResourceType().Serialize(),
		},
		Relation:    relation.Serialize(),
		Subject:     subjectToV1Beta1(subject),
		Pagination:  paginationToV1Beta1(pagination),
		Consistency: consistencyToV1Beta1(consistency),
	}, opts...)
	if err != nil {
		a.incrFailureCounter("LookupObjects")
		return nil, err
	}

	return &lookupObjectsStream{stream: stream}, nil
}

func (a *GRPCRelationsRepository) LookupSubjects(ctx context.Context,
	object model.ResourceReference, relation model.Relation,
	subjectType model.RepresentationType,
	subjectRelation *model.Relation,
	pagination *model.Pagination, consistency model.Consistency,
) (model.ResultStream[model.LookupSubjectsItem], error) {
	opts, err := a.getCallOptions()
	if err != nil {
		a.incrFailureCounter("LookupSubjects")
		return nil, err
	}

	subReporterType, err := subjectType.RequireReporterType()
	if err != nil {
		a.incrFailureCounter("LookupSubjects")
		return nil, fmt.Errorf("LookupSubjects requires subject reporter type: %w", err)
	}

	req := &kesselapi.LookupSubjectsRequest{
		Resource: resourceReferenceToV1Beta1(object),
		Relation: relation.Serialize(),
		SubjectType: &kesselapi.ObjectType{
			Namespace: subReporterType.Serialize(),
			Name:      subjectType.ResourceType().Serialize(),
		},
		Pagination:  paginationToV1Beta1(pagination),
		Consistency: consistencyToV1Beta1(consistency),
	}

	if subjectRelation != nil {
		rel := subjectRelation.Serialize()
		req.SubjectRelation = &rel
	}

	stream, err := a.LookupService.LookupSubjects(ctx, req, opts...)
	if err != nil {
		a.incrFailureCounter("LookupSubjects")
		return nil, err
	}

	return &lookupSubjectsStream{stream: stream}, nil
}

func (a *GRPCRelationsRepository) CreateTuples(ctx context.Context, tuples []model.RelationsTuple, upsert bool, fencing *model.FencingCheck,
) (model.ConsistencyToken, error) {
	log.Infof("Creating tuples: %d", len(tuples))
	opts, err := a.getCallOptions()
	if err != nil {
		a.incrFailureCounter("CreateTuples")
		return model.MinimizeLatencyToken, err
	}

	req := &kesselapi.CreateTuplesRequest{
		Upsert: upsert,
		Tuples: tuplesToV1Beta1(tuples),
	}
	if fencing != nil {
		req.FencingCheck = fencingCheckToV1Beta1(fencing)
	}

	resp, err := a.TupleService.CreateTuples(ctx, req, opts...)
	if err != nil {
		a.incrFailureCounter("CreateTuples")
		return model.MinimizeLatencyToken, err
	}

	a.incrSuccessCounter("CreateTuples")
	return tokenFromV1Beta1(resp.GetConsistencyToken()), nil
}

func (a *GRPCRelationsRepository) DeleteTuples(ctx context.Context, filter model.TupleFilter, fencing *model.FencingCheck,
) (model.ConsistencyToken, error) {
	opts, err := a.getCallOptions()
	if err != nil {
		a.incrFailureCounter("DeleteTuples")
		return model.MinimizeLatencyToken, err
	}

	req := &kesselapi.DeleteTuplesRequest{
		Filter: tupleFilterToV1Beta1(filter),
	}
	if fencing != nil {
		req.FencingCheck = fencingCheckToV1Beta1(fencing)
	}

	resp, err := a.TupleService.DeleteTuples(ctx, req, opts...)
	if err != nil {
		a.incrFailureCounter("DeleteTuples")
		return model.MinimizeLatencyToken, err
	}

	a.incrSuccessCounter("DeleteTuples")
	return tokenFromV1Beta1(resp.GetConsistencyToken()), nil
}

func (a *GRPCRelationsRepository) ReadTuples(ctx context.Context, filter model.TupleFilter, pagination *model.Pagination, consistency model.Consistency,
) (model.ResultStream[model.ReadTuplesItem], error) {
	opts, err := a.getCallOptions()
	if err != nil {
		a.incrFailureCounter("ReadTuples")
		return nil, err
	}

	stream, err := a.TupleService.ReadTuples(ctx, &kesselapi.ReadTuplesRequest{
		Filter:      tupleFilterToV1Beta1(filter),
		Pagination:  paginationToV1Beta1(pagination),
		Consistency: consistencyToV1Beta1(consistency),
	}, opts...)
	if err != nil {
		a.incrFailureCounter("ReadTuples")
		return nil, err
	}

	a.incrSuccessCounter("ReadTuples")
	return &readTuplesStream{stream: stream}, nil
}

func (a *GRPCRelationsRepository) AcquireLock(ctx context.Context, lockId model.LockId) (model.LockToken, error) {
	opts, err := a.getCallOptions()
	if err != nil {
		a.incrFailureCounter("AcquireLock")
		return model.LockToken(""), err
	}

	resp, err := a.TupleService.AcquireLock(ctx, &kesselapi.AcquireLockRequest{
		LockId: lockId.String(),
	}, opts...)
	if err != nil {
		a.incrFailureCounter("AcquireLock")
		return model.LockToken(""), err
	}

	a.incrSuccessCounter("AcquireLock")
	return model.DeserializeLockToken(resp.GetLockToken()), nil
}

// --- protobuf conversion helpers ---

func resourceReferenceToV1Beta1(ref model.ResourceReference) *kesselapi.ObjectReference {
	objType := &kesselapi.ObjectType{
		Name: ref.ResourceType().Serialize(),
	}
	if ref.HasReporter() {
		objType.Namespace = ref.Reporter().ReporterType().Serialize()
	}
	return &kesselapi.ObjectReference{
		Type: objType,
		Id:   ref.ResourceId().Serialize(),
	}
}

func subjectToV1Beta1(sub model.SubjectReference) *kesselapi.SubjectReference {
	subResource := sub.Resource()
	ref := &kesselapi.SubjectReference{
		Subject: resourceReferenceToV1Beta1(subResource),
	}
	if sub.HasRelation() {
		relation := sub.Relation().Serialize()
		ref.Relation = &relation
	}
	return ref
}

func consistencyToV1Beta1(c model.Consistency) *kesselapi.Consistency {
	if token := model.ConsistencyAtLeastAsFreshToken(c); token != nil {
		return &kesselapi.Consistency{
			Requirement: &kesselapi.Consistency_AtLeastAsFresh{
				AtLeastAsFresh: &kesselapi.ConsistencyToken{
					Token: token.Serialize(),
				},
			},
		}
	}
	return &kesselapi.Consistency{
		Requirement: &kesselapi.Consistency_MinimizeLatency{
			MinimizeLatency: true,
		},
	}
}

func tokenFromV1Beta1(t *kesselapi.ConsistencyToken) model.ConsistencyToken {
	if t == nil {
		return model.MinimizeLatencyToken
	}
	return model.DeserializeConsistencyToken(t.GetToken())
}

func paginationToV1Beta1(pagination *model.Pagination) *kesselapi.RequestPagination {
	if pagination == nil {
		return nil
	}
	result := &kesselapi.RequestPagination{
		Limit: pagination.Limit,
	}
	if pagination.Continuation != nil {
		result.ContinuationToken = pagination.Continuation
	}
	return result
}

func fencingCheckToV1Beta1(fencing *model.FencingCheck) *kesselapi.FencingCheck {
	return &kesselapi.FencingCheck{
		LockId:    fencing.LockId().String(),
		LockToken: fencing.LockToken().String(),
	}
}

func relationshipsToCheckBulkV1Beta1(rels []model.Relationship) []*kesselapi.CheckBulkRequestItem {
	protoItems := make([]*kesselapi.CheckBulkRequestItem, len(rels))
	for i, rel := range rels {
		protoItems[i] = &kesselapi.CheckBulkRequestItem{
			Resource: resourceReferenceToV1Beta1(rel.Object()),
			Relation: rel.Relation().Serialize(),
			Subject:  subjectToV1Beta1(rel.Subject()),
		}
	}
	return protoItems
}

func checkBulkResultFromV1Beta1(respPairs []*kesselapi.CheckBulkResponsePair, rels []model.Relationship, protoToken *kesselapi.ConsistencyToken) (model.CheckBulkResult, error) {
	if len(respPairs) != len(rels) {
		return model.CheckBulkResult{}, status.Errorf(codes.Internal,
			"internal error: mismatched backend check results: expected %d pairs, got %d", len(rels), len(respPairs))
	}

	pairs := make([]model.CheckBulkResultPair, len(respPairs))
	for i, pair := range respPairs {
		var resultItem model.CheckBulkResultItem
		if pair.GetError() != nil {
			resultItem = model.NewCheckBulkResultItem(
				false,
				fmt.Errorf("check failed: %s", pair.GetError().GetMessage()),
				pair.GetError().GetCode(),
			)
		} else if pair.GetItem() != nil {
			resultItem = model.NewCheckBulkResultItem(
				pair.GetItem().GetAllowed() == kesselapi.CheckBulkResponseItem_ALLOWED_TRUE,
				nil, 0,
			)
		} else {
			resultItem = model.NewCheckBulkResultItem(
				false,
				fmt.Errorf("malformed backend response: both error and item are nil for pair %v", pair),
				int32(codes.Internal),
			)
		}

		pairs[i] = model.NewCheckBulkResultPair(rels[i], resultItem)
	}

	return model.NewCheckBulkResult(pairs, tokenFromV1Beta1(protoToken)), nil
}

func tuplesToV1Beta1(tuples []model.RelationsTuple) []*kesselapi.Relationship {
	relationships := make([]*kesselapi.Relationship, len(tuples))
	for i, tuple := range tuples {
		relationships[i] = &kesselapi.Relationship{
			Resource: resourceReferenceToV1Beta1(tuple.Object()),
			Relation: tuple.Relation().Serialize(),
			Subject:  subjectToV1Beta1(tuple.Subject()),
		}
	}
	return relationships
}

func tupleFilterToV1Beta1(filter model.TupleFilter) *kesselapi.RelationTupleFilter {
	result := &kesselapi.RelationTupleFilter{}
	if filter.ReporterType() != nil {
		result.ResourceNamespace = proto.String(filter.ReporterType().Serialize())
	}
	if filter.ObjectType() != nil {
		result.ResourceType = proto.String(filter.ObjectType().Serialize())
	}
	if filter.ObjectId() != nil {
		result.ResourceId = proto.String(filter.ObjectId().Serialize())
	}
	if filter.Relation() != nil {
		result.Relation = proto.String(filter.Relation().Serialize())
	}
	if filter.Subject() != nil {
		sf := &kesselapi.SubjectFilter{}
		if filter.Subject().ReporterType() != nil {
			sf.SubjectNamespace = proto.String(filter.Subject().ReporterType().Serialize())
		}
		if filter.Subject().SubjectType() != nil {
			sf.SubjectType = proto.String(filter.Subject().SubjectType().Serialize())
		}
		if filter.Subject().SubjectId() != nil {
			sf.SubjectId = proto.String(filter.Subject().SubjectId().Serialize())
		}
		if filter.Subject().Relation() != nil {
			sf.Relation = proto.String(filter.Subject().Relation().Serialize())
		}
		result.SubjectFilter = sf
	}
	return result
}

// --- streaming adapters ---

type lookupObjectsStream struct {
	stream grpc.ServerStreamingClient[kesselapi.LookupResourcesResponse]
}

func (s *lookupObjectsStream) Recv() (model.LookupObjectsItem, error) {
	resp, err := s.stream.Recv()
	if err != nil {
		return model.LookupObjectsItem{}, err
	}
	reporterType := model.DeserializeReporterType(resp.GetResource().GetType().GetNamespace())
	reporter := model.NewReporterReference(reporterType, nil)
	object := model.NewResourceReference(
		model.DeserializeResourceType(resp.GetResource().GetType().GetName()),
		model.DeserializeLocalResourceId(resp.GetResource().GetId()),
		&reporter,
	)
	return model.NewLookupObjectsItem(object, resp.GetPagination().GetContinuationToken()), nil
}

type lookupSubjectsStream struct {
	stream grpc.ServerStreamingClient[kesselapi.LookupSubjectsResponse]
}

func (s *lookupSubjectsStream) Recv() (model.LookupSubjectsItem, error) {
	resp, err := s.stream.Recv()
	if err != nil {
		return model.LookupSubjectsItem{}, err
	}

	subProto := resp.GetSubject()
	subjectObj := subProto.GetSubject()
	reporterType := model.DeserializeReporterType(subjectObj.GetType().GetNamespace())
	reporter := model.NewReporterReference(reporterType, nil)
	subResource := model.NewResourceReference(
		model.DeserializeResourceType(subjectObj.GetType().GetName()),
		model.DeserializeLocalResourceId(subjectObj.GetId()),
		&reporter,
	)

	var subjectRef model.SubjectReference
	if subProto.Relation != nil {
		rel := model.DeserializeRelation(*subProto.Relation)
		subjectRef = model.NewSubjectReference(subResource, &rel)
	} else {
		subjectRef = model.NewSubjectReferenceWithoutRelation(subResource)
	}

	return model.NewLookupSubjectsItem(subjectRef, resp.GetPagination().GetContinuationToken()), nil
}

type emptyLookupObjectsStream struct{}

func (s *emptyLookupObjectsStream) Recv() (model.LookupObjectsItem, error) {
	return model.LookupObjectsItem{}, io.EOF
}

type emptyLookupSubjectsStream struct{}

func (s *emptyLookupSubjectsStream) Recv() (model.LookupSubjectsItem, error) {
	return model.LookupSubjectsItem{}, io.EOF
}

type readTuplesStream struct {
	stream grpc.ServerStreamingClient[kesselapi.ReadTuplesResponse]
}

func (s *readTuplesStream) Recv() (model.ReadTuplesItem, error) {
	resp, err := s.stream.Recv()
	if err != nil {
		return model.ReadTuplesItem{}, err
	}
	tuple := resp.GetTuple()

	objReporterType := model.DeserializeReporterType(tuple.GetResource().GetType().GetNamespace())
	objReporter := model.NewReporterReference(objReporterType, nil)
	object := model.NewResourceReference(
		model.DeserializeResourceType(tuple.GetResource().GetType().GetName()),
		model.DeserializeLocalResourceId(tuple.GetResource().GetId()),
		&objReporter,
	)

	subReporterType := model.DeserializeReporterType(tuple.GetSubject().GetSubject().GetType().GetNamespace())
	subReporter := model.NewReporterReference(subReporterType, nil)
	subResource := model.NewResourceReference(
		model.DeserializeResourceType(tuple.GetSubject().GetSubject().GetType().GetName()),
		model.DeserializeLocalResourceId(tuple.GetSubject().GetSubject().GetId()),
		&subReporter,
	)

	var subjectRef model.SubjectReference
	if tuple.GetSubject().Relation != nil {
		rel := model.DeserializeRelation(*tuple.GetSubject().Relation)
		subjectRef = model.NewSubjectReference(subResource, &rel)
	} else {
		subjectRef = model.NewSubjectReferenceWithoutRelation(subResource)
	}

	var consistencyToken model.ConsistencyToken
	if token := resp.GetConsistencyToken().GetToken(); token != "" {
		consistencyToken = model.DeserializeConsistencyToken(token)
	}
	return model.NewReadTuplesItem(
		object,
		model.DeserializeRelation(tuple.GetRelation()),
		subjectRef,
		resp.GetPagination().GetContinuationToken(),
		consistencyToken,
	), nil
}

type emptyReadTuplesStream struct{}

func (s *emptyReadTuplesStream) Recv() (model.ReadTuplesItem, error) {
	return model.ReadTuplesItem{}, io.EOF
}
