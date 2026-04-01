package data

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/go-kratos/kratos/v2/log"
	kesselv1 "github.com/project-kessel/relations-api/api/kessel/relations/v1"
	kessel "github.com/project-kessel/relations-api/api/kessel/relations/v1beta1"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"google.golang.org/grpc"

	"github.com/project-kessel/inventory-api/internal/biz/model"
)

type spicedbRelationsRepository struct {
	healthService  kesselv1.KesselRelationsHealthServiceClient
	checkService   kessel.KesselCheckServiceClient
	tupleService   kessel.KesselTupleServiceClient
	lookupService  kessel.KesselLookupServiceClient
	tokenClient    *relationsTokenClient
	logger         *log.Helper
	successCounter metric.Int64Counter
	failureCounter metric.Int64Counter
}

var _ model.RelationsRepository = &spicedbRelationsRepository{}

func newSpicedbRelationsRepository(_ context.Context, config RelationsCompletedConfig, logger *log.Helper) (*spicedbRelationsRepository, error) {
	logger.Info("Using relations repository: spicedb")
	tokenCli := newRelationsTokenClient(config.tokenConfig)

	meter := otel.Meter("github.com/project-kessel/inventory-api/blob/main/internal/server/otel")

	successCounter, err := meter.Int64Counter("inventory_relations_api_success")
	if err != nil {
		return nil, fmt.Errorf("failed to create success counter: %w", err)
	}

	failureCounter, err := meter.Int64Counter("inventory_relations_api_failure")
	if err != nil {
		return nil, fmt.Errorf("failed to create failure counter: %w", err)
	}

	return &spicedbRelationsRepository{
		healthService:  kesselv1.NewKesselRelationsHealthServiceClient(config.gRPCConn),
		checkService:   kessel.NewKesselCheckServiceClient(config.gRPCConn),
		tupleService:   kessel.NewKesselTupleServiceClient(config.gRPCConn),
		lookupService:  kessel.NewKesselLookupServiceClient(config.gRPCConn),
		logger:         logger,
		tokenClient:    tokenCli,
		successCounter: successCounter,
		failureCounter: failureCounter,
	}, nil
}

func (r *spicedbRelationsRepository) incrFailureCounter(method string) {
	r.failureCounter.Add(context.Background(), 1, metric.WithAttributes(
		attribute.String("method", method),
	))
}

func (r *spicedbRelationsRepository) incrSuccessCounter(method string) {
	r.successCounter.Add(context.Background(), 1, metric.WithAttributes(attribute.String("method", method)))
}

func (r *spicedbRelationsRepository) getCallOptions() ([]grpc.CallOption, error) {
	var opts []grpc.CallOption
	opts = append(opts, grpc.EmptyCallOption{})
	if r.tokenClient.EnableOIDCAuth {
		token, err := r.tokenClient.getToken()
		if err != nil {
			return nil, fmt.Errorf("failed to request token: %w", err)
		}
		if r.tokenClient.Insecure {
			opts = append(opts, withInsecureBearerToken(token.AccessToken))
		} else {
			opts = append(opts, withBearerToken(token.AccessToken))
		}
	}
	return opts, nil
}

func (r *spicedbRelationsRepository) Health(ctx context.Context) error {
	opts, err := r.getCallOptions()
	if err != nil {
		r.incrFailureCounter("Health")
		return err
	}
	_, err = r.healthService.GetReadyz(ctx, &kesselv1.GetReadyzRequest{}, opts...)
	if err != nil {
		r.incrFailureCounter("Health")
		return err
	}
	r.incrSuccessCounter("Health")
	return nil
}

func (r *spicedbRelationsRepository) Check(ctx context.Context, resource model.ReporterResourceKey, relation model.Relation,
	subject model.SubjectReference, consistency model.Consistency) (bool, model.ConsistencyToken, error) {

	opts, err := r.getCallOptions()
	if err != nil {
		r.incrFailureCounter("Check")
		return false, "", err
	}

	resp, err := r.checkService.Check(ctx, &kessel.CheckRequest{
		Resource:    reporterResourceKeyToObjectRef(resource),
		Relation:    relation.Serialize(),
		Subject:     subjectRefToV1Beta1(subject),
		Consistency: consistencyToV1Beta1(consistency),
	}, opts...)
	if err != nil {
		r.incrFailureCounter("Check")
		return false, "", err
	}

	r.incrSuccessCounter("Check")
	token := tokenFromV1Beta1(resp.GetConsistencyToken())
	return resp.GetAllowed() == kessel.CheckResponse_ALLOWED_TRUE, token, nil
}

func (r *spicedbRelationsRepository) CheckForUpdate(ctx context.Context, resource model.ReporterResourceKey, relation model.Relation,
	subject model.SubjectReference) (bool, model.ConsistencyToken, error) {

	opts, err := r.getCallOptions()
	if err != nil {
		r.incrFailureCounter("CheckForUpdate")
		return false, "", err
	}

	resp, err := r.checkService.CheckForUpdate(ctx, &kessel.CheckForUpdateRequest{
		Resource: reporterResourceKeyToObjectRef(resource),
		Relation: relation.Serialize(),
		Subject:  subjectRefToV1Beta1(subject),
	}, opts...)
	if err != nil {
		r.incrFailureCounter("CheckForUpdate")
		return false, "", err
	}

	r.incrSuccessCounter("CheckForUpdate")
	token := tokenFromV1Beta1(resp.GetConsistencyToken())
	return resp.GetAllowed() == kessel.CheckForUpdateResponse_ALLOWED_TRUE, token, nil
}

func (r *spicedbRelationsRepository) CheckBulk(ctx context.Context, items []model.CheckItem,
	consistency model.Consistency) ([]model.CheckBulkResultItem, model.ConsistencyToken, error) {

	v1Items := make([]*kessel.CheckBulkRequestItem, len(items))
	for i, item := range items {
		v1Items[i] = &kessel.CheckBulkRequestItem{
			Resource: reporterResourceKeyToObjectRef(item.Resource),
			Relation: item.Relation.Serialize(),
			Subject:  subjectRefToV1Beta1(item.Subject),
		}
	}

	opts, err := r.getCallOptions()
	if err != nil {
		r.incrFailureCounter("CheckBulk")
		return nil, "", err
	}

	resp, err := r.checkService.CheckBulk(ctx, &kessel.CheckBulkRequest{
		Items:       v1Items,
		Consistency: consistencyToV1Beta1(consistency),
	}, opts...)
	if err != nil {
		r.incrFailureCounter("CheckBulk")
		return nil, "", err
	}

	pairs := resp.GetPairs()
	results := make([]model.CheckBulkResultItem, len(pairs))
	for i, pair := range pairs {
		if pair.GetError() != nil {
			results[i] = model.CheckBulkResultItem{
				Allowed: false,
				Error:   fmt.Errorf("check failed: %s", pair.GetError().GetMessage()),
			}
		} else if pair.GetItem() != nil {
			results[i] = model.CheckBulkResultItem{
				Allowed: pair.GetItem().GetAllowed() == kessel.CheckBulkResponseItem_ALLOWED_TRUE,
			}
		}
	}

	r.incrSuccessCounter("CheckBulk")
	token := tokenFromV1Beta1(resp.GetConsistencyToken())
	return results, token, nil
}

func (r *spicedbRelationsRepository) CheckForUpdateBulk(ctx context.Context, items []model.CheckItem) ([]model.CheckBulkResultItem, model.ConsistencyToken, error) {
	v1Items := make([]*kessel.CheckBulkRequestItem, len(items))
	for i, item := range items {
		v1Items[i] = &kessel.CheckBulkRequestItem{
			Resource: reporterResourceKeyToObjectRef(item.Resource),
			Relation: item.Relation.Serialize(),
			Subject:  subjectRefToV1Beta1(item.Subject),
		}
	}

	opts, err := r.getCallOptions()
	if err != nil {
		r.incrFailureCounter("CheckForUpdateBulk")
		return nil, "", err
	}

	resp, err := r.checkService.CheckForUpdateBulk(ctx, &kessel.CheckForUpdateBulkRequest{
		Items: v1Items,
	}, opts...)
	if err != nil {
		r.incrFailureCounter("CheckForUpdateBulk")
		return nil, "", err
	}

	pairs := resp.GetPairs()
	results := make([]model.CheckBulkResultItem, len(pairs))
	for i, pair := range pairs {
		if pair.GetError() != nil {
			results[i] = model.CheckBulkResultItem{
				Allowed: false,
				Error:   fmt.Errorf("check for update failed: %s", pair.GetError().GetMessage()),
			}
		} else if pair.GetItem() != nil {
			results[i] = model.CheckBulkResultItem{
				Allowed: pair.GetItem().GetAllowed() == kessel.CheckBulkResponseItem_ALLOWED_TRUE,
			}
		}
	}

	r.incrSuccessCounter("CheckForUpdateBulk")
	token := tokenFromV1Beta1(resp.GetConsistencyToken())
	return results, token, nil
}

func (r *spicedbRelationsRepository) LookupResources(ctx context.Context, query model.LookupResourcesQuery) (model.LookupResourcesIterator, error) {
	var continuationToken *string
	if query.Continuation != "" {
		continuationToken = &query.Continuation
	}

	opts, err := r.getCallOptions()
	if err != nil {
		r.incrFailureCounter("LookupResources")
		return nil, err
	}

	stream, err := r.lookupService.LookupResources(ctx, &kessel.LookupResourcesRequest{
		ResourceType: &kessel.ObjectType{
			Namespace: query.ReporterType.Serialize(),
			Name:      query.ResourceType.Serialize(),
		},
		Relation: query.Relation.Serialize(),
		Subject:  subjectRefToV1Beta1(query.Subject),
		Pagination: &kessel.RequestPagination{
			Limit:             query.Limit,
			ContinuationToken: continuationToken,
		},
		Consistency: consistencyToV1Beta1(query.Consistency),
	}, opts...)
	if err != nil {
		r.incrFailureCounter("LookupResources")
		return nil, err
	}

	r.incrSuccessCounter("LookupResources")
	return &relationsLookupResourcesIterator{stream: stream}, nil
}

func (r *spicedbRelationsRepository) LookupSubjects(ctx context.Context, query model.LookupSubjectsQuery) (model.LookupSubjectsIterator, error) {
	opts, err := r.getCallOptions()
	if err != nil {
		r.incrFailureCounter("LookupSubjects")
		return nil, err
	}

	var pagination *kessel.RequestPagination
	if query.Limit != 0 || query.Continuation != "" {
		pagination = &kessel.RequestPagination{Limit: query.Limit}
		if query.Continuation != "" {
			c := query.Continuation
			pagination.ContinuationToken = &c
		}
	}

	req := &kessel.LookupSubjectsRequest{
		Resource: reporterResourceKeyToObjectRef(query.Resource),
		Relation: query.Relation.Serialize(),
		SubjectType: &kessel.ObjectType{
			Namespace: query.SubjectReporter.Serialize(),
			Name:      query.SubjectType.Serialize(),
		},
		Pagination:  pagination,
		Consistency: consistencyToV1Beta1(query.Consistency),
	}
	if query.SubjectRelation != nil {
		sr := query.SubjectRelation.Serialize()
		req.SubjectRelation = &sr
	}

	stream, err := r.lookupService.LookupSubjects(ctx, req, opts...)
	if err != nil {
		r.incrFailureCounter("LookupSubjects")
		return nil, err
	}

	r.incrSuccessCounter("LookupSubjects")
	return &relationsLookupSubjectsIterator{stream: stream}, nil
}

func (r *spicedbRelationsRepository) CreateTuples(ctx context.Context, tuples []model.RelationsTuple, upsert bool,
	lockId, lockToken string) (model.ConsistencyToken, error) {

	rels := tuplesToV1Beta1Relationships(tuples)

	opts, err := r.getCallOptions()
	if err != nil {
		r.incrFailureCounter("CreateTuples")
		return "", err
	}

	req := &kessel.CreateTuplesRequest{
		Upsert: upsert,
		Tuples: rels,
	}
	if lockId != "" {
		req.FencingCheck = &kessel.FencingCheck{
			LockId:    lockId,
			LockToken: lockToken,
		}
	}

	resp, err := r.tupleService.CreateTuples(ctx, req, opts...)
	if err != nil {
		r.incrFailureCounter("CreateTuples")
		return "", err
	}

	r.incrSuccessCounter("CreateTuples")
	return tokenFromV1Beta1(resp.GetConsistencyToken()), nil
}

func (r *spicedbRelationsRepository) DeleteTuples(ctx context.Context, tuples []model.RelationsTuple,
	lockId, lockToken string) (model.ConsistencyToken, error) {

	opts, err := r.getCallOptions()
	if err != nil {
		r.incrFailureCounter("DeleteTuples")
		return "", err
	}

	for _, tuple := range tuples {
		filter := tupleToV1Beta1Filter(tuple)
		req := &kessel.DeleteTuplesRequest{
			Filter: filter,
		}
		if lockId != "" {
			req.FencingCheck = &kessel.FencingCheck{
				LockId:    lockId,
				LockToken: lockToken,
			}
		}

		_, err := r.tupleService.DeleteTuples(ctx, req, opts...)
		if err != nil {
			r.incrFailureCounter("DeleteTuples")
			return "", err
		}
	}

	r.incrSuccessCounter("DeleteTuples")
	return "", nil
}

func (r *spicedbRelationsRepository) AcquireLock(ctx context.Context, lockId string) (string, error) {
	opts, err := r.getCallOptions()
	if err != nil {
		r.incrFailureCounter("AcquireLock")
		return "", err
	}

	resp, err := r.tupleService.AcquireLock(ctx, &kessel.AcquireLockRequest{
		LockId: lockId,
	}, opts...)
	if err != nil {
		r.incrFailureCounter("AcquireLock")
		return "", err
	}

	r.incrSuccessCounter("AcquireLock")
	return resp.GetLockToken(), nil
}

// --- v1beta1 conversion helpers (package-private) ---

func reporterResourceKeyToObjectRef(key model.ReporterResourceKey) *kessel.ObjectReference {
	return &kessel.ObjectReference{
		Type: &kessel.ObjectType{
			Namespace: key.ReporterType().Serialize(),
			Name:      key.ResourceType().Serialize(),
		},
		Id: key.LocalResourceId().Serialize(),
	}
}

func subjectReferenceFromV1Beta1(sub *kessel.SubjectReference) (model.SubjectReference, error) {
	if sub == nil || sub.Subject == nil || sub.Subject.Type == nil {
		return model.SubjectReference{}, fmt.Errorf("invalid v1beta1 subject reference")
	}
	localID, err := model.NewLocalResourceId(sub.Subject.GetId())
	if err != nil {
		return model.SubjectReference{}, fmt.Errorf("local resource id: %w", err)
	}
	resType, err := model.NewResourceType(sub.Subject.Type.GetName())
	if err != nil {
		return model.SubjectReference{}, err
	}
	repType, err := model.NewReporterType(sub.Subject.Type.GetNamespace())
	if err != nil {
		return model.SubjectReference{}, err
	}
	key, err := model.NewReporterResourceKey(localID, resType, repType, model.ReporterInstanceId(""))
	if err != nil {
		return model.SubjectReference{}, err
	}
	if relStr := sub.GetRelation(); relStr != "" {
		rel, err := model.NewRelation(relStr)
		if err != nil {
			return model.SubjectReference{}, err
		}
		return model.NewSubjectReference(key, &rel), nil
	}
	return model.NewSubjectReferenceWithoutRelation(key), nil
}

func subjectRefToV1Beta1(sub model.SubjectReference) *kessel.SubjectReference {
	subKey := sub.Subject()
	ref := &kessel.SubjectReference{
		Subject: &kessel.ObjectReference{
			Type: &kessel.ObjectType{
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

func consistencyToV1Beta1(c model.Consistency) *kessel.Consistency {
	if token := model.ConsistencyAtLeastAsFreshToken(c); token != nil {
		return &kessel.Consistency{
			Requirement: &kessel.Consistency_AtLeastAsFresh{
				AtLeastAsFresh: &kessel.ConsistencyToken{
					Token: token.Serialize(),
				},
			},
		}
	}
	return &kessel.Consistency{
		Requirement: &kessel.Consistency_MinimizeLatency{MinimizeLatency: true},
	}
}

func tokenFromV1Beta1(ct *kessel.ConsistencyToken) model.ConsistencyToken {
	if ct == nil {
		return ""
	}
	return model.DeserializeConsistencyToken(ct.GetToken())
}

func tuplesToV1Beta1Relationships(tuples []model.RelationsTuple) []*kessel.Relationship {
	rels := make([]*kessel.Relationship, len(tuples))
	for i, tuple := range tuples {
		rels[i] = tupleToV1Beta1Relationship(tuple)
	}
	return rels
}

func tupleToV1Beta1Relationship(tuple model.RelationsTuple) *kessel.Relationship {
	return &kessel.Relationship{
		Resource: &kessel.ObjectReference{
			Type: &kessel.ObjectType{
				Name:      tuple.Resource().Type().Name(),
				Namespace: tuple.Resource().Type().Namespace(),
			},
			Id: tuple.Resource().Id().Serialize(),
		},
		Relation: tuple.Relation(),
		Subject: &kessel.SubjectReference{
			Subject: &kessel.ObjectReference{
				Type: &kessel.ObjectType{
					Name:      tuple.Subject().Subject().Type().Name(),
					Namespace: tuple.Subject().Subject().Type().Namespace(),
				},
				Id: tuple.Subject().Subject().Id().Serialize(),
			},
		},
	}
}

func tupleToV1Beta1Filter(tuple model.RelationsTuple) *kessel.RelationTupleFilter {
	resourceNamespace := tuple.Resource().Type().Namespace()
	resourceType := tuple.Resource().Type().Name()
	resourceId := tuple.Resource().Id().Serialize()
	relation := tuple.Relation()
	subjectNamespace := tuple.Subject().Subject().Type().Namespace()
	subjectType := tuple.Subject().Subject().Type().Name()
	subjectId := tuple.Subject().Subject().Id().Serialize()

	return &kessel.RelationTupleFilter{
		ResourceNamespace: &resourceNamespace,
		ResourceType:      &resourceType,
		ResourceId:        &resourceId,
		Relation:          &relation,
		SubjectFilter: &kessel.SubjectFilter{
			SubjectNamespace: &subjectNamespace,
			SubjectType:      &subjectType,
			SubjectId:        &subjectId,
		},
	}
}

// relationsLookupResourcesIterator wraps a gRPC streaming client as a LookupResourcesIterator.
type relationsLookupResourcesIterator struct {
	stream grpc.ServerStreamingClient[kessel.LookupResourcesResponse]
}

func (it *relationsLookupResourcesIterator) Next() (*model.LookupResourceResult, error) {
	resp, err := it.stream.Recv()
	if err == io.EOF {
		return nil, io.EOF
	}
	if err != nil {
		return nil, err
	}

	resType, _ := model.NewResourceType(resp.Resource.Type.Name)
	namespace, _ := model.NewReporterType(resp.Resource.Type.Namespace)
	resourceId, _ := model.NewLocalResourceId(resp.Resource.Id)

	var contToken string
	if resp.Pagination != nil {
		contToken = strings.TrimSpace(resp.Pagination.ContinuationToken)
	}

	return &model.LookupResourceResult{
		ResourceId:        resourceId,
		ResourceType:      resType,
		Namespace:         namespace,
		ContinuationToken: contToken,
	}, nil
}

// relationsLookupSubjectsIterator wraps a gRPC streaming client as a LookupSubjectsIterator.
type relationsLookupSubjectsIterator struct {
	stream grpc.ServerStreamingClient[kessel.LookupSubjectsResponse]
}

func (it *relationsLookupSubjectsIterator) Next() (*model.LookupSubjectResult, error) {
	resp, err := it.stream.Recv()
	if err == io.EOF {
		return nil, io.EOF
	}
	if err != nil {
		return nil, err
	}

	subj, err := subjectReferenceFromV1Beta1(resp.GetSubject())
	if err != nil {
		return nil, err
	}

	var contToken string
	if resp.Pagination != nil {
		contToken = strings.TrimSpace(resp.Pagination.ContinuationToken)
	}

	return &model.LookupSubjectResult{
		Subject:           subj,
		ContinuationToken: contToken,
	}, nil
}
