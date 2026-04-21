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
	return model.HealthResult{
		Status: resp.GetStatus(),
		Code:   int(resp.GetCode()),
	}, nil
}

func (a *GRPCRelationsRepository) Check(ctx context.Context, resource model.ReporterResourceKey, relation model.Relation,
	subject model.SubjectReference, consistency model.Consistency,
) (model.CheckResult, error) {
	namespace := resource.ReporterType().Serialize()
	resourceType := resource.ResourceType().Serialize()
	localResourceId := resource.LocalResourceId().Serialize()

	log.Infof("Check: on resourceType=%s, localResourceId=%s", resourceType, localResourceId)

	opts, err := a.getCallOptions()
	if err != nil {
		a.incrFailureCounter("Check")
		return model.CheckResult{}, err
	}

	resp, err := a.CheckService.Check(ctx, &kesselapi.CheckRequest{
		Resource: &kesselapi.ObjectReference{
			Type: &kesselapi.ObjectType{
				Namespace: namespace,
				Name:      resourceType,
			},
			Id: localResourceId,
		},
		Relation:    relation.Serialize(),
		Subject:     subjectToV1Beta1(subject),
		Consistency: consistencyToV1Beta1(consistency),
	}, opts...)

	if err != nil {
		a.incrFailureCounter("Check")
		return model.CheckResult{}, err
	}

	a.incrSuccessCounter("Check")
	return model.CheckResult{
		Allowed:          resp.GetAllowed() == kesselapi.CheckResponse_ALLOWED_TRUE,
		ConsistencyToken: tokenFromV1Beta1(resp.GetConsistencyToken()),
	}, nil
}

func (a *GRPCRelationsRepository) CheckForUpdate(ctx context.Context, resource model.ReporterResourceKey, relation model.Relation,
	subject model.SubjectReference,
) (model.CheckResult, error) {
	opts, err := a.getCallOptions()
	if err != nil {
		a.incrFailureCounter("CheckForUpdate")
		return model.CheckResult{}, err
	}

	resp, err := a.CheckService.CheckForUpdate(ctx, &kesselapi.CheckForUpdateRequest{
		Resource: &kesselapi.ObjectReference{
			Type: &kesselapi.ObjectType{
				Namespace: resource.ReporterType().Serialize(),
				Name:      resource.ResourceType().Serialize(),
			},
			Id: resource.LocalResourceId().Serialize(),
		},
		Relation: relation.Serialize(),
		Subject:  subjectToV1Beta1(subject),
	}, opts...)

	if err != nil {
		a.incrFailureCounter("CheckForUpdate")
		return model.CheckResult{}, err
	}

	a.incrSuccessCounter("CheckForUpdate")
	return model.CheckResult{
		Allowed:          resp.GetAllowed() == kesselapi.CheckForUpdateResponse_ALLOWED_TRUE,
		ConsistencyToken: tokenFromV1Beta1(resp.GetConsistencyToken()),
	}, nil
}

func (a *GRPCRelationsRepository) CheckBulk(ctx context.Context, items []model.CheckBulkItem, consistency model.Consistency,
) (model.CheckBulkResult, error) {
	log.Infof("CheckBulk: checking %d items", len(items))

	protoItems := checkBulkItemsToV1Beta1(items)
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
	return checkBulkResultFromV1Beta1(resp.GetPairs(), items, resp.GetConsistencyToken())
}

func (a *GRPCRelationsRepository) CheckForUpdateBulk(ctx context.Context, items []model.CheckBulkItem,
) (model.CheckBulkResult, error) {
	log.Infof("CheckForUpdateBulk: checking %d items", len(items))

	protoItems := checkBulkItemsToV1Beta1(items)

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
	return checkBulkResultFromV1Beta1(resp.GetPairs(), items, resp.GetConsistencyToken())
}

func (a *GRPCRelationsRepository) LookupResources(ctx context.Context, resourceType model.ResourceType, reporterType model.ReporterType,
	relation model.Relation, subject model.SubjectReference, pagination *model.Pagination, consistency model.Consistency,
) (model.ResultStream[model.LookupResourcesItem], error) {
	opts, err := a.getCallOptions()
	if err != nil {
		a.incrFailureCounter("LookupResources")
		return nil, err
	}

	stream, err := a.LookupService.LookupResources(ctx, &kesselapi.LookupResourcesRequest{
		ResourceType: &kesselapi.ObjectType{
			Namespace: reporterType.Serialize(),
			Name:      resourceType.Serialize(),
		},
		Relation:    relation.Serialize(),
		Subject:     subjectToV1Beta1(subject),
		Pagination:  paginationToV1Beta1(pagination),
		Consistency: consistencyToV1Beta1(consistency),
	}, opts...)
	if err != nil {
		a.incrFailureCounter("LookupResources")
		return nil, err
	}

	return &lookupResourcesStream{stream: stream}, nil
}

func (a *GRPCRelationsRepository) LookupSubjects(ctx context.Context, resource model.ReporterResourceKey, relation model.Relation,
	subjectType model.ResourceType, subjectReporter model.ReporterType, subjectRelation *model.Relation,
	pagination *model.Pagination, consistency model.Consistency,
) (model.ResultStream[model.LookupSubjectsItem], error) {
	opts, err := a.getCallOptions()
	if err != nil {
		a.incrFailureCounter("LookupSubjects")
		return nil, err
	}

	req := &kesselapi.LookupSubjectsRequest{
		Resource: &kesselapi.ObjectReference{
			Type: &kesselapi.ObjectType{
				Namespace: resource.ReporterType().Serialize(),
				Name:      resource.ResourceType().Serialize(),
			},
			Id: resource.LocalResourceId().Serialize(),
		},
		Relation: relation.Serialize(),
		SubjectType: &kesselapi.ObjectType{
			Namespace: subjectReporter.Serialize(),
			Name:      subjectType.Serialize(),
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
) (model.TuplesResult, error) {
	log.Infof("Creating tuples: %d", len(tuples))
	opts, err := a.getCallOptions()
	if err != nil {
		a.incrFailureCounter("CreateTuples")
		return model.TuplesResult{}, err
	}

	req := &kesselapi.CreateTuplesRequest{
		Upsert: upsert,
		Tuples: tuplesToV1Beta1(tuples),
	}
	if fencing != nil {
		req.FencingCheck = &kesselapi.FencingCheck{
			LockId:    fencing.LockId,
			LockToken: fencing.LockToken,
		}
	}

	resp, err := a.TupleService.CreateTuples(ctx, req, opts...)
	if err != nil {
		a.incrFailureCounter("CreateTuples")
		return model.TuplesResult{}, err
	}

	a.incrSuccessCounter("CreateTuples")
	return model.TuplesResult{
		ConsistencyToken: tokenFromV1Beta1(resp.GetConsistencyToken()),
	}, nil
}

func (a *GRPCRelationsRepository) DeleteTuples(ctx context.Context, tuples []model.RelationsTuple, fencing *model.FencingCheck,
) (model.TuplesResult, error) {
	opts, err := a.getCallOptions()
	if err != nil {
		a.incrFailureCounter("DeleteTuples")
		return model.TuplesResult{}, err
	}

	req := &kesselapi.DeleteTuplesRequest{
		Filter: tupleToFilter(tuples[0]),
	}
	if fencing != nil {
		req.FencingCheck = &kesselapi.FencingCheck{
			LockId:    fencing.LockId,
			LockToken: fencing.LockToken,
		}
	}

	resp, err := a.TupleService.DeleteTuples(ctx, req, opts...)
	if err != nil {
		a.incrFailureCounter("DeleteTuples")
		return model.TuplesResult{}, err
	}

	a.incrSuccessCounter("DeleteTuples")
	return model.TuplesResult{
		ConsistencyToken: tokenFromV1Beta1(resp.GetConsistencyToken()),
	}, nil
}

func (a *GRPCRelationsRepository) DeleteTuplesByFilter(ctx context.Context, filter model.TupleFilter, fencing *model.FencingCheck,
) (model.TuplesResult, error) {
	opts, err := a.getCallOptions()
	if err != nil {
		a.incrFailureCounter("DeleteTuplesByFilter")
		return model.TuplesResult{}, err
	}

	req := &kesselapi.DeleteTuplesRequest{
		Filter: tupleFilterToV1Beta1(filter),
	}
	if fencing != nil {
		req.FencingCheck = &kesselapi.FencingCheck{
			LockId:    fencing.LockId,
			LockToken: fencing.LockToken,
		}
	}

	resp, err := a.TupleService.DeleteTuples(ctx, req, opts...)
	if err != nil {
		a.incrFailureCounter("DeleteTuplesByFilter")
		return model.TuplesResult{}, err
	}

	a.incrSuccessCounter("DeleteTuplesByFilter")
	return model.TuplesResult{
		ConsistencyToken: tokenFromV1Beta1(resp.GetConsistencyToken()),
	}, nil
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

func (a *GRPCRelationsRepository) AcquireLock(ctx context.Context, lockId string) (model.AcquireLockResult, error) {
	opts, err := a.getCallOptions()
	if err != nil {
		a.incrFailureCounter("AcquireLock")
		return model.AcquireLockResult{}, err
	}

	resp, err := a.TupleService.AcquireLock(ctx, &kesselapi.AcquireLockRequest{
		LockId: lockId,
	}, opts...)
	if err != nil {
		a.incrFailureCounter("AcquireLock")
		return model.AcquireLockResult{}, err
	}

	a.incrSuccessCounter("AcquireLock")
	return model.AcquireLockResult{
		LockToken: resp.GetLockToken(),
	}, nil
}

// --- protobuf conversion helpers ---

func subjectToV1Beta1(sub model.SubjectReference) *kesselapi.SubjectReference {
	subKey := sub.Subject()
	ref := &kesselapi.SubjectReference{
		Subject: &kesselapi.ObjectReference{
			Type: &kesselapi.ObjectType{
				Namespace: subKey.ReporterType().Serialize(),
				Name:      subKey.ResourceType().Serialize(),
			},
			Id: subKey.LocalResourceId().Serialize(),
		},
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

func checkBulkItemsToV1Beta1(items []model.CheckBulkItem) []*kesselapi.CheckBulkRequestItem {
	protoItems := make([]*kesselapi.CheckBulkRequestItem, len(items))
	for i, item := range items {
		protoItems[i] = &kesselapi.CheckBulkRequestItem{
			Resource: &kesselapi.ObjectReference{
				Type: &kesselapi.ObjectType{
					Namespace: item.Resource.ReporterType().Serialize(),
					Name:      item.Resource.ResourceType().Serialize(),
				},
				Id: item.Resource.LocalResourceId().Serialize(),
			},
			Relation: item.Relation.Serialize(),
			Subject:  subjectToV1Beta1(item.Subject),
		}
	}
	return protoItems
}

func checkBulkResultFromV1Beta1(respPairs []*kesselapi.CheckBulkResponsePair, items []model.CheckBulkItem, protoToken *kesselapi.ConsistencyToken) (model.CheckBulkResult, error) {
	if len(respPairs) != len(items) {
		return model.CheckBulkResult{}, status.Errorf(codes.Internal,
			"internal error: mismatched backend check results: expected %d pairs, got %d", len(items), len(respPairs))
	}

	pairs := make([]model.CheckBulkResultPair, len(respPairs))
	for i, pair := range respPairs {
		var resultItem model.CheckBulkResultItem
		if pair.GetError() != nil {
			resultItem = model.CheckBulkResultItem{
				Allowed:   false,
				Error:     fmt.Errorf("check failed: %s", pair.GetError().GetMessage()),
				ErrorCode: pair.GetError().GetCode(),
			}
		} else if pair.GetItem() != nil {
			resultItem = model.CheckBulkResultItem{
				Allowed: pair.GetItem().GetAllowed() == kesselapi.CheckBulkResponseItem_ALLOWED_TRUE,
			}
		} else {
			resultItem = model.CheckBulkResultItem{
				Allowed:   false,
				Error:     fmt.Errorf("malformed backend response: both error and item are nil for pair %v", pair),
				ErrorCode: int32(codes.Internal),
			}
		}

		pairs[i] = model.CheckBulkResultPair{
			Request: items[i],
			Result:  resultItem,
		}
	}

	return model.CheckBulkResult{
		Pairs:            pairs,
		ConsistencyToken: tokenFromV1Beta1(protoToken),
	}, nil
}

func tuplesToV1Beta1(tuples []model.RelationsTuple) []*kesselapi.Relationship {
	relationships := make([]*kesselapi.Relationship, len(tuples))
	for i, tuple := range tuples {
		relationships[i] = &kesselapi.Relationship{
			Resource: &kesselapi.ObjectReference{
				Type: &kesselapi.ObjectType{
					Name:      tuple.Resource().Type().Name(),
					Namespace: tuple.Resource().Type().Namespace(),
				},
				Id: tuple.Resource().Id().Serialize(),
			},
			Relation: tuple.Relation().Serialize(),
			Subject: &kesselapi.SubjectReference{
				Subject: &kesselapi.ObjectReference{
					Type: &kesselapi.ObjectType{
						Name:      tuple.Subject().Subject().Type().Name(),
						Namespace: tuple.Subject().Subject().Type().Namespace(),
					},
					Id: tuple.Subject().Subject().Id().Serialize(),
				},
			},
		}
	}
	return relationships
}

func tupleToFilter(tuple model.RelationsTuple) *kesselapi.RelationTupleFilter {
	resourceNamespace := tuple.Resource().Type().Namespace()
	resourceType := tuple.Resource().Type().Name()
	resourceId := tuple.Resource().Id().Serialize()
	relation := tuple.Relation().Serialize()
	subjectNamespace := tuple.Subject().Subject().Type().Namespace()
	subjectType := tuple.Subject().Subject().Type().Name()
	subjectId := tuple.Subject().Subject().Id().Serialize()

	filter := &kesselapi.RelationTupleFilter{
		ResourceNamespace: proto.String(resourceNamespace),
		ResourceType:      proto.String(resourceType),
		ResourceId:        proto.String(resourceId),
		Relation:          proto.String(relation),
		SubjectFilter: &kesselapi.SubjectFilter{
			SubjectNamespace: proto.String(subjectNamespace),
			SubjectType:      proto.String(subjectType),
			SubjectId:        proto.String(subjectId),
		},
	}
	if tuple.Subject().HasRelation() {
		filter.SubjectFilter.Relation = proto.String(tuple.Subject().Relation().Serialize())
	} else {
		filter.SubjectFilter.Relation = proto.String("")
	}
	return filter
}

// --- streaming adapters ---

type lookupResourcesStream struct {
	stream grpc.ServerStreamingClient[kesselapi.LookupResourcesResponse]
}

func (s *lookupResourcesStream) Recv() (model.LookupResourcesItem, error) {
	resp, err := s.stream.Recv()
	if err != nil {
		return model.LookupResourcesItem{}, err
	}
	return model.LookupResourcesItem{
		ResourceId:        model.DeserializeLocalResourceId(resp.GetResource().GetId()),
		ResourceType:      model.DeserializeResourceType(resp.GetResource().GetType().GetName()),
		ReporterType:      model.DeserializeReporterType(resp.GetResource().GetType().GetNamespace()),
		ContinuationToken: resp.GetPagination().GetContinuationToken(),
	}, nil
}

type lookupSubjectsStream struct {
	stream grpc.ServerStreamingClient[kesselapi.LookupSubjectsResponse]
}

func (s *lookupSubjectsStream) Recv() (model.LookupSubjectsItem, error) {
	resp, err := s.stream.Recv()
	if err != nil {
		return model.LookupSubjectsItem{}, err
	}

	item := model.LookupSubjectsItem{
		SubjectId:         model.DeserializeLocalResourceId(resp.GetSubject().GetSubject().GetId()),
		SubjectType:       model.DeserializeResourceType(resp.GetSubject().GetSubject().GetType().GetName()),
		SubjectReporter:   model.DeserializeReporterType(resp.GetSubject().GetSubject().GetType().GetNamespace()),
		ContinuationToken: resp.GetPagination().GetContinuationToken(),
	}
	if resp.GetSubject().Relation != nil {
		rel := model.DeserializeRelation(*resp.GetSubject().Relation)
		item.SubjectRelation = &rel
	}

	return item, nil
}

// emptyLookupResourcesStream implements ResultStream for an empty result set.
type emptyLookupResourcesStream struct{}

func (s *emptyLookupResourcesStream) Recv() (model.LookupResourcesItem, error) {
	return model.LookupResourcesItem{}, io.EOF
}

// emptyLookupSubjectsStream implements ResultStream for an empty result set.
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
	item := model.ReadTuplesItem{
		ResourceNamespace: tuple.GetResource().GetType().GetNamespace(),
		ResourceType:      tuple.GetResource().GetType().GetName(),
		ResourceId:        tuple.GetResource().GetId(),
		Relation:          tuple.GetRelation(),
		SubjectNamespace:  tuple.GetSubject().GetSubject().GetType().GetNamespace(),
		SubjectType:       tuple.GetSubject().GetSubject().GetType().GetName(),
		SubjectId:         tuple.GetSubject().GetSubject().GetId(),
		ContinuationToken: resp.GetPagination().GetContinuationToken(),
	}
	if tuple.GetSubject().Relation != nil {
		item.SubjectRelation = tuple.GetSubject().Relation
	}
	if token := resp.GetConsistencyToken().GetToken(); token != "" {
		item.ConsistencyToken = model.DeserializeConsistencyToken(token)
	}
	return item, nil
}

// emptyReadTuplesStream implements ResultStream for an empty result set.
type emptyReadTuplesStream struct{}

func (s *emptyReadTuplesStream) Recv() (model.ReadTuplesItem, error) {
	return model.ReadTuplesItem{}, io.EOF
}

func tupleFilterToV1Beta1(filter model.TupleFilter) *kesselapi.RelationTupleFilter {
	result := &kesselapi.RelationTupleFilter{
		ResourceNamespace: filter.ResourceNamespace,
		ResourceType:      filter.ResourceType,
		ResourceId:        filter.ResourceId,
		Relation:          filter.Relation,
	}
	if filter.SubjectFilter != nil {
		result.SubjectFilter = &kesselapi.SubjectFilter{
			SubjectNamespace: filter.SubjectFilter.SubjectNamespace,
			SubjectType:      filter.SubjectFilter.SubjectType,
			SubjectId:        filter.SubjectFilter.SubjectId,
			Relation:         filter.SubjectFilter.Relation,
		}
	}
	return result
}
