package data

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	v1 "github.com/authzed/authzed-go/proto/authzed/api/v1"
	"github.com/authzed/authzed-go/v1"
	"github.com/authzed/grpcutil"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/uuid"
	"github.com/project-kessel/inventory-api/internal/biz/model"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health/grpc_health_v1"
)

const (
	relationPrefix      = "t_"
	lockType            = "kessel/lock"
	lockVersionType     = "kessel/lockversion"
	lockVersionRelation = "version"
)

// SpiceDBRelationsRepository implements the Relations Repository interface using SpiceDB.
type SpiceDBRelationsRepository struct {
	client                   *authzed.Client
	healthClient             grpc_health_v1.HealthClient
	schemaFilePath           string
	initOnce                 sync.Once
	initErr                  error
	fullyConsistentAsDefault bool
	Logger                   *log.Helper
	successCounter           metric.Int64Counter
	failureCounter           metric.Int64Counter
	cleanup                  func()
}

// NewSpiceDBRelationsRepository creates a new SpiceDB repository instance.
func NewSpiceDBRelationsRepository(config *SpiceDBConfig, logger log.Logger) (*SpiceDBRelationsRepository, func(), error) {
	logHelper := log.NewHelper(logger)
	logHelper.Info("creating spicedb connection")

	var opts []grpc.DialOption
	opts = append(opts, grpc.EmptyDialOption{})

	var token string
	var err error
	if config.TokenFile != "" {
		token, err = readFile(config.TokenFile)
		if err != nil {
			logHelper.Error(err)
			return nil, nil, fmt.Errorf("error creating spicedb client: error loading token file: %w", err)
		}
	} else {
		token = config.Token
	}

	if token == "" {
		return nil, nil, fmt.Errorf("error creating spicedb client: token is empty")
	}

	if !config.UseTLS {
		opts = append(opts, grpcutil.WithInsecureBearerToken(token))
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	} else {
		tlsConfig, _ := grpcutil.WithSystemCerts(grpcutil.VerifyCA)
		opts = append(opts, grpcutil.WithBearerToken(token))
		opts = append(opts, tlsConfig)
	}

	client, err := authzed.NewClient(
		config.Endpoint,
		opts...,
	)

	if err != nil {
		return nil, nil, fmt.Errorf("error creating spicedb client: %w", err)
	}

	// Create health client
	conn, err := grpc.NewClient(
		config.Endpoint,
		opts...,
	)
	if err != nil {
		return nil, nil, fmt.Errorf("error creating grpc health client: %w", err)
	}
	healthClient := grpc_health_v1.NewHealthClient(conn)

	// Initialize metrics
	meter := otel.Meter("github.com/project-kessel/inventory-api/blob/main/internal/server/otel")

	successCounter, err := meter.Int64Counter("inventory_spicedb_success")
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create success counter: %w", err)
	}

	failureCounter, err := meter.Int64Counter("inventory_spicedb_failure")
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create failure counter: %w", err)
	}

	cleanup := func() {
		if err := client.Close(); err != nil {
			logHelper.Errorf("error closing spicedb client: %v", err)
		}
		if err := conn.Close(); err != nil {
			logHelper.Errorf("error closing spicedb health connection: %v", err)
		}
	}

	repo := &SpiceDBRelationsRepository{
		client:                   client,
		healthClient:             healthClient,
		schemaFilePath:           config.SchemaFile,
		fullyConsistentAsDefault: config.FullyConsistent,
		Logger:                   logHelper,
		successCounter:           successCounter,
		failureCounter:           failureCounter,
		cleanup:                  cleanup,
	}

	return repo, cleanup, nil
}

func (s *SpiceDBRelationsRepository) incrFailureCounter(method string) {
	s.failureCounter.Add(context.Background(), 1, metric.WithAttributes(
		attribute.String("method", method),
	))
}

func (s *SpiceDBRelationsRepository) incrSuccessCounter(method string) {
	s.successCounter.Add(context.Background(), 1, metric.WithAttributes(attribute.String("method", method)))
}

func (s *SpiceDBRelationsRepository) initialize(ctx context.Context) error {
	s.initOnce.Do(func() {
		schema, err := readFile(s.schemaFilePath)
		if err != nil {
			s.initErr = fmt.Errorf("failed to load schema file: %w", err)
			return
		}

		_, err = s.client.WriteSchema(ctx, &v1.WriteSchemaRequest{
			Schema: schema,
		})
		if err != nil {
			s.initErr = err
		}
	})
	return s.initErr
}

// Close closes the SpiceDB client connections.
func (s *SpiceDBRelationsRepository) Close() {
	if s.cleanup != nil {
		s.cleanup()
	}
}

// Health checks the health of the SpiceDB backend.
func (s *SpiceDBRelationsRepository) Health(ctx context.Context) (model.HealthResult, error) {
	timeoutCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	resp, err := s.healthClient.Check(timeoutCtx, &grpc_health_v1.HealthCheckRequest{})
	if err != nil {
		s.incrFailureCounter("Health")
		return model.HealthResult{}, err
	}

	select {
	case <-timeoutCtx.Done():
		s.incrFailureCounter("Health")
		return model.HealthResult{}, fmt.Errorf("timeout connecting to backend")
	default:
		switch resp.Status {
		case grpc_health_v1.HealthCheckResponse_NOT_SERVING:
			s.incrFailureCounter("Health")
			return model.NewHealthResult("NOT_SERVING", 503), nil
		case grpc_health_v1.HealthCheckResponse_SERVICE_UNKNOWN:
			s.incrFailureCounter("Health")
			return model.NewHealthResult("SERVICE_UNKNOWN", 503), nil
		case grpc_health_v1.HealthCheckResponse_SERVING:
			s.incrSuccessCounter("Health")
			return model.NewHealthResult("SERVING", 200), nil
		}
	}
	s.incrFailureCounter("Health")
	return model.HealthResult{}, fmt.Errorf("error connecting to backend")
}

// Check performs a single permission check.
func (s *SpiceDBRelationsRepository) Check(ctx context.Context, rel model.Relationship, consistency model.Consistency) (model.CheckResult, error) {
	if err := s.initialize(ctx); err != nil {
		s.incrFailureCounter("Check")
		return model.CheckResult{}, err
	}

	obj := rel.Object()
	subj := rel.Subject()

	log.Infof("Check: on resourceType=%s, localResourceId=%s",
		obj.ResourceType().Serialize(), obj.ResourceId().Serialize())

	req := &v1.CheckPermissionRequest{
		Consistency: s.determineConsistency(consistency),
		Resource: &v1.ObjectReference{
			ObjectType: resourceReferenceToSpiceDBType(obj),
			ObjectId:   obj.ResourceId().Serialize(),
		},
		Permission: rel.Relation().Serialize(),
		Subject: &v1.SubjectReference{
			Object: &v1.ObjectReference{
				ObjectType: resourceReferenceToSpiceDBType(subj.Resource()),
				ObjectId:   subj.Resource().ResourceId().Serialize(),
			},
			OptionalRelation: optionalRelationToString(subj.Relation()),
		},
	}

	checkResponse, err := s.client.CheckPermission(ctx, req)
	if err != nil {
		s.incrFailureCounter("Check")
		return model.CheckResult{}, fmt.Errorf("error invoking CheckPermission in SpiceDB: %w", err)
	}

	allowed := checkResponse.Permissionship == v1.CheckPermissionResponse_PERMISSIONSHIP_HAS_PERMISSION
	token := model.DeserializeConsistencyToken(checkResponse.GetCheckedAt().GetToken())

	s.incrSuccessCounter("Check")
	return model.NewCheckResult(allowed, token), nil
}

// CheckForUpdate performs a strongly-consistent permission check.
func (s *SpiceDBRelationsRepository) CheckForUpdate(ctx context.Context, rel model.Relationship) (model.CheckResult, error) {
	if err := s.initialize(ctx); err != nil {
		s.incrFailureCounter("CheckForUpdate")
		return model.CheckResult{}, err
	}

	obj := rel.Object()
	subj := rel.Subject()

	req := &v1.CheckPermissionRequest{
		Consistency: &v1.Consistency{Requirement: &v1.Consistency_FullyConsistent{FullyConsistent: true}},
		Resource: &v1.ObjectReference{
			ObjectType: resourceReferenceToSpiceDBType(obj),
			ObjectId:   obj.ResourceId().Serialize(),
		},
		Permission: rel.Relation().Serialize(),
		Subject: &v1.SubjectReference{
			Object: &v1.ObjectReference{
				ObjectType: resourceReferenceToSpiceDBType(subj.Resource()),
				ObjectId:   subj.Resource().ResourceId().Serialize(),
			},
			OptionalRelation: optionalRelationToString(subj.Relation()),
		},
	}

	checkResponse, err := s.client.CheckPermission(ctx, req)
	if err != nil {
		s.incrFailureCounter("CheckForUpdate")
		return model.CheckResult{}, fmt.Errorf("error invoking CheckPermission in SpiceDB: %w", err)
	}

	allowed := checkResponse.Permissionship == v1.CheckPermissionResponse_PERMISSIONSHIP_HAS_PERMISSION
	token := model.DeserializeConsistencyToken(checkResponse.GetCheckedAt().GetToken())

	s.incrSuccessCounter("CheckForUpdate")
	return model.NewCheckResult(allowed, token), nil
}

// CheckBulk performs multiple permission checks in one request.
func (s *SpiceDBRelationsRepository) CheckBulk(ctx context.Context, rels []model.Relationship, consistency model.Consistency) (model.CheckBulkResult, error) {
	if err := s.initialize(ctx); err != nil {
		s.incrFailureCounter("CheckBulk")
		return model.CheckBulkResult{}, err
	}

	log.Infof("CheckBulk: checking %d items", len(rels))

	items := make([]*v1.CheckBulkPermissionsRequestItem, len(rels))
	for i, rel := range rels {
		items[i] = relationshipToCheckBulkItem(rel)
	}

	req := &v1.CheckBulkPermissionsRequest{
		Consistency: s.determineConsistency(consistency),
		Items:       items,
	}

	resp, err := s.client.CheckBulkPermissions(ctx, req)
	if err != nil {
		s.incrFailureCounter("CheckBulk")
		return model.CheckBulkResult{}, fmt.Errorf("error invoking CheckBulkPermissions in SpiceDB: %w", err)
	}

	pairs := make([]model.CheckBulkResultPair, len(resp.Pairs))
	for i, p := range resp.Pairs {
		pairs[i] = spicePairToCheckBulkResultPair(p, rels[i])
	}

	token := model.DeserializeConsistencyToken(resp.GetCheckedAt().GetToken())
	s.incrSuccessCounter("CheckBulk")
	return model.NewCheckBulkResult(pairs, token), nil
}

// CheckForUpdateBulk performs multiple strongly-consistent permission checks.
func (s *SpiceDBRelationsRepository) CheckForUpdateBulk(ctx context.Context, rels []model.Relationship) (model.CheckBulkResult, error) {
	if err := s.initialize(ctx); err != nil {
		s.incrFailureCounter("CheckForUpdateBulk")
		return model.CheckBulkResult{}, err
	}

	log.Infof("CheckForUpdateBulk: checking %d items", len(rels))

	items := make([]*v1.CheckBulkPermissionsRequestItem, len(rels))
	for i, rel := range rels {
		items[i] = relationshipToCheckBulkItem(rel)
	}

	req := &v1.CheckBulkPermissionsRequest{
		Consistency: &v1.Consistency{Requirement: &v1.Consistency_FullyConsistent{FullyConsistent: true}},
		Items:       items,
	}

	resp, err := s.client.CheckBulkPermissions(ctx, req)
	if err != nil {
		s.incrFailureCounter("CheckForUpdateBulk")
		return model.CheckBulkResult{}, fmt.Errorf("error invoking CheckBulkPermissions in SpiceDB (CheckForUpdateBulk): %w", err)
	}

	pairs := make([]model.CheckBulkResultPair, len(resp.Pairs))
	for i, p := range resp.Pairs {
		pairs[i] = spicePairToCheckBulkResultPair(p, rels[i])
	}

	token := model.DeserializeConsistencyToken(resp.GetCheckedAt().GetToken())
	s.incrSuccessCounter("CheckForUpdateBulk")
	return model.NewCheckBulkResult(pairs, token), nil
}

// LookupObjects looks up resources accessible by a subject.
func (s *SpiceDBRelationsRepository) LookupObjects(
	ctx context.Context,
	objectType model.RepresentationType,
	relation model.Relation,
	subject model.SubjectReference,
	pagination *model.Pagination,
	consistency model.Consistency,
) (model.ResultStream[model.LookupObjectsItem], error) {
	if err := s.initialize(ctx); err != nil {
		s.incrFailureCounter("LookupObjects")
		return nil, err
	}

	var cursor *v1.Cursor
	if pagination != nil && pagination.Continuation != nil {
		cursor = &v1.Cursor{Token: pagination.Continuation.Serialize()}
	}

	var limit uint32
	if pagination != nil {
		limit = pagination.Limit
	}

	req := &v1.LookupResourcesRequest{
		Consistency:        s.determineConsistency(consistency),
		ResourceObjectType: representationTypeToSpiceDBType(objectType),
		Permission:         relation.Serialize(),
		Subject: &v1.SubjectReference{
			Object: &v1.ObjectReference{
				ObjectType: resourceReferenceToSpiceDBType(subject.Resource()),
				ObjectId:   subject.Resource().ResourceId().Serialize(),
			},
			OptionalRelation: optionalRelationToString(subject.Relation()),
		},
		OptionalLimit:  limit,
		OptionalCursor: cursor,
	}

	client, err := s.client.LookupResources(ctx, req)
	if err != nil {
		s.incrFailureCounter("LookupObjects")
		return nil, fmt.Errorf("error invoking LookupResources in SpiceDB: %w", err)
	}

	return newSpicedbLookupObjectsStream(client, objectType), nil
}

// LookupSubjects looks up subjects that have access to a resource.
func (s *SpiceDBRelationsRepository) LookupSubjects(
	ctx context.Context,
	object model.ResourceReference,
	relation model.Relation,
	subjectType model.RepresentationType,
	subjectRelation *model.Relation,
	pagination *model.Pagination,
	consistency model.Consistency,
) (model.ResultStream[model.LookupSubjectsItem], error) {
	if err := s.initialize(ctx); err != nil {
		s.incrFailureCounter("LookupSubjects")
		return nil, err
	}

	var cursor *v1.Cursor
	if pagination != nil && pagination.Continuation != nil {
		cursor = &v1.Cursor{Token: pagination.Continuation.Serialize()}
	}

	var limit uint32
	if pagination != nil {
		limit = pagination.Limit
	}

	req := &v1.LookupSubjectsRequest{
		Consistency: s.determineConsistency(consistency),
		Resource: &v1.ObjectReference{
			ObjectType: resourceReferenceToSpiceDBType(object),
			ObjectId:   object.ResourceId().Serialize(),
		},
		Permission:              relation.Serialize(),
		SubjectObjectType:       representationTypeToSpiceDBType(subjectType),
		WildcardOption:          v1.LookupSubjectsRequest_WILDCARD_OPTION_EXCLUDE_WILDCARDS,
		OptionalSubjectRelation: optionalRelationToString(subjectRelation),
		OptionalConcreteLimit:   limit,
		OptionalCursor:          cursor,
	}

	client, err := s.client.LookupSubjects(ctx, req)
	if err != nil {
		s.incrFailureCounter("LookupSubjects")
		return nil, fmt.Errorf("error invoking LookupSubjects in SpiceDB: %w", err)
	}

	return newSpicedbLookupSubjectsStream(client, subjectType), nil
}

// CreateTuples creates or updates relationship tuples.
func (s *SpiceDBRelationsRepository) CreateTuples(
	ctx context.Context,
	tuples []model.RelationsTuple,
	upsert bool,
	fencing *model.FencingCheck,
) (model.TuplesResult, error) {
	if err := s.initialize(ctx); err != nil {
		s.incrFailureCounter("CreateTuples")
		return model.TuplesResult{}, err
	}

	log.Infof("Creating tuples: %d", len(tuples))

	var relationshipUpdates []*v1.RelationshipUpdate

	operation := v1.RelationshipUpdate_OPERATION_CREATE
	if upsert {
		operation = v1.RelationshipUpdate_OPERATION_TOUCH
	}

	for _, tuple := range tuples {
		relationshipUpdates = append(relationshipUpdates, &v1.RelationshipUpdate{
			Operation:    operation,
			Relationship: relationsTupleToSpiceDBRelationship(tuple),
		})
	}

	req := &v1.WriteRelationshipsRequest{
		Updates: relationshipUpdates,
	}

	if fencing != nil {
		req.OptionalPreconditions = []*v1.Precondition{
			{
				Operation: v1.Precondition_OPERATION_MUST_MATCH,
				Filter: &v1.RelationshipFilter{
					ResourceType:       lockType,
					OptionalResourceId: fencing.LockId().Serialize(),
					OptionalRelation:   addRelationPrefix(lockVersionRelation, relationPrefix),
					OptionalSubjectFilter: &v1.SubjectFilter{
						SubjectType:       lockVersionType,
						OptionalSubjectId: fencing.LockToken().Serialize(),
					},
				},
			},
		}
	}

	resp, err := s.client.WriteRelationships(ctx, req)
	if err != nil {
		s.incrFailureCounter("CreateTuples")
		return model.TuplesResult{}, fmt.Errorf("error writing relationships to SpiceDB: %w", err)
	}

	token := model.DeserializeConsistencyToken(resp.GetWrittenAt().GetToken())
	s.incrSuccessCounter("CreateTuples")
	return model.NewTuplesResult(token), nil
}

// DeleteTuples deletes relationship tuples matching a filter.
func (s *SpiceDBRelationsRepository) DeleteTuples(
	ctx context.Context,
	filter model.TupleFilter,
	fencing *model.FencingCheck,
) (model.TuplesResult, error) {
	if err := s.initialize(ctx); err != nil {
		s.incrFailureCounter("DeleteTuples")
		return model.TuplesResult{}, err
	}

	relationshipFilter, err := tupleFilterToSpiceDBFilter(filter)
	if err != nil {
		s.incrFailureCounter("DeleteTuples")
		return model.TuplesResult{}, fmt.Errorf("SpiceDB request validation: %w", err)
	}

	req := &v1.DeleteRelationshipsRequest{
		RelationshipFilter: relationshipFilter,
	}

	if fencing != nil {
		req.OptionalPreconditions = []*v1.Precondition{
			{
				Operation: v1.Precondition_OPERATION_MUST_MATCH,
				Filter: &v1.RelationshipFilter{
					ResourceType:       lockType,
					OptionalResourceId: fencing.LockId().Serialize(),
					OptionalRelation:   addRelationPrefix(lockVersionRelation, relationPrefix),
					OptionalSubjectFilter: &v1.SubjectFilter{
						SubjectType:       lockVersionType,
						OptionalSubjectId: fencing.LockToken().Serialize(),
					},
				},
			},
		}
	}

	resp, err := s.client.DeleteRelationships(ctx, req)
	if err != nil {
		s.incrFailureCounter("DeleteTuples")
		return model.TuplesResult{}, fmt.Errorf("error invoking DeleteRelationships in SpiceDB: %w", err)
	}

	token := model.DeserializeConsistencyToken(resp.GetDeletedAt().GetToken())
	s.incrSuccessCounter("DeleteTuples")
	return model.NewTuplesResult(token), nil
}

// ReadTuples reads relationship tuples matching a filter.
func (s *SpiceDBRelationsRepository) ReadTuples(
	ctx context.Context,
	filter model.TupleFilter,
	pagination *model.Pagination,
	consistency model.Consistency,
) (model.ResultStream[model.ReadTuplesItem], error) {
	if err := s.initialize(ctx); err != nil {
		s.incrFailureCounter("ReadTuples")
		return nil, err
	}

	var cursor *v1.Cursor
	if pagination != nil && pagination.Continuation != nil {
		cursor = &v1.Cursor{Token: pagination.Continuation.Serialize()}
	}

	var limit uint32
	if pagination != nil {
		limit = pagination.Limit
	}

	relationshipFilter, err := tupleFilterToSpiceDBFilter(filter)
	if err != nil {
		s.incrFailureCounter("ReadTuples")
		return nil, fmt.Errorf("SpiceDB request validation: %w", err)
	}

	req := &v1.ReadRelationshipsRequest{
		Consistency:        s.determineConsistency(consistency),
		RelationshipFilter: relationshipFilter,
		OptionalLimit:      limit,
		OptionalCursor:     cursor,
	}

	client, err := s.client.ReadRelationships(ctx, req)
	if err != nil {
		s.incrFailureCounter("ReadTuples")
		return nil, fmt.Errorf("error invoking ReadRelationships in SpiceDB: %w", err)
	}

	s.incrSuccessCounter("ReadTuples")
	return newSpicedbReadTuplesStream(client), nil
}

// AcquireLock acquires a distributed lock.
func (s *SpiceDBRelationsRepository) AcquireLock(ctx context.Context, lockId model.LockId) (model.AcquireLockResult, error) {
	if err := s.initialize(ctx); err != nil {
		s.incrFailureCounter("AcquireLock")
		return model.AcquireLockResult{}, err
	}

	newFencingToken := uuid.New().String()

	readClient, err := s.client.ReadRelationships(ctx, &v1.ReadRelationshipsRequest{
		Consistency: &v1.Consistency{Requirement: &v1.Consistency_FullyConsistent{FullyConsistent: true}},
		RelationshipFilter: &v1.RelationshipFilter{
			ResourceType:       lockType,
			OptionalResourceId: lockId.Serialize(),
			OptionalRelation:   addRelationPrefix(lockVersionRelation, relationPrefix),
		},
	})
	if err != nil {
		s.incrFailureCounter("AcquireLock")
		return model.AcquireLockResult{}, fmt.Errorf("error invoking ReadRelationships in SpiceDB: %w", err)
	}

	existingLock, err := readClient.Recv()
	if err != nil && !errors.Is(err, io.EOF) {
		s.incrFailureCounter("AcquireLock")
		return model.AcquireLockResult{}, fmt.Errorf("error reading existing lock: %w", err)
	}

	var updates []*v1.RelationshipUpdate
	var preconditions []*v1.Precondition

	if existingLock != nil && existingLock.Relationship != nil {
		updates = append(updates, &v1.RelationshipUpdate{
			Operation:    v1.RelationshipUpdate_OPERATION_DELETE,
			Relationship: existingLock.Relationship,
		})

		preconditions = append(preconditions, &v1.Precondition{
			Operation: v1.Precondition_OPERATION_MUST_MATCH,
			Filter: &v1.RelationshipFilter{
				ResourceType:       existingLock.Relationship.Resource.ObjectType,
				OptionalResourceId: existingLock.Relationship.Resource.ObjectId,
				OptionalRelation:   existingLock.Relationship.Relation,
				OptionalSubjectFilter: &v1.SubjectFilter{
					SubjectType:       existingLock.Relationship.Subject.Object.ObjectType,
					OptionalSubjectId: existingLock.Relationship.Subject.Object.ObjectId,
				},
			},
		})
	}

	updates = append(updates, &v1.RelationshipUpdate{
		Operation: v1.RelationshipUpdate_OPERATION_CREATE,
		Relationship: &v1.Relationship{
			Resource: &v1.ObjectReference{
				ObjectType: lockType,
				ObjectId:   lockId.Serialize(),
			},
			Relation: addRelationPrefix(lockVersionRelation, relationPrefix),
			Subject: &v1.SubjectReference{
				Object: &v1.ObjectReference{
					ObjectType: lockVersionType,
					ObjectId:   newFencingToken,
				},
			},
		},
	})

	_, err = s.client.WriteRelationships(ctx, &v1.WriteRelationshipsRequest{
		Updates:               updates,
		OptionalPreconditions: preconditions,
	})
	if err != nil {
		s.incrFailureCounter("AcquireLock")
		return model.AcquireLockResult{}, fmt.Errorf("error writing relationships to SpiceDB: %w", err)
	}

	token := model.DeserializeLockToken(newFencingToken)
	s.incrSuccessCounter("AcquireLock")
	return model.NewAcquireLockResult(token), nil
}

// Helper functions for type conversions

func resourceReferenceToSpiceDBType(ref model.ResourceReference) string {
	if !ref.HasReporter() {
		return ref.ResourceType().Serialize()
	}
	return fmt.Sprintf("%s/%s", ref.Reporter().ReporterType().Serialize(), ref.ResourceType().Serialize())
}

func representationTypeToSpiceDBType(rt model.RepresentationType) string {
	if !rt.HasReporterType() {
		return rt.ResourceType().Serialize()
	}
	return fmt.Sprintf("%s/%s", rt.ReporterType().Serialize(), rt.ResourceType().Serialize())
}

func spiceDBTypeToResourceReference(spicedbType string, id string) (model.ResourceReference, error) {
	parts := strings.Split(spicedbType, "/")

	var resourceType model.ResourceType
	var reporter *model.ReporterReference

	switch len(parts) {
	case 1:
		resourceType = model.DeserializeResourceType(parts[0])
		reporter = nil
	case 2:
		reporterType := model.DeserializeReporterType(parts[0])
		resourceType = model.DeserializeResourceType(parts[1])
		rep := model.NewReporterReference(reporterType, nil)
		reporter = &rep
	default:
		return model.ResourceReference{}, fmt.Errorf("invalid spicedb type: %s", spicedbType)
	}

	resourceId := model.DeserializeLocalResourceId(id)
	return model.NewResourceReference(resourceType, resourceId, reporter), nil
}

func relationshipToCheckBulkItem(rel model.Relationship) *v1.CheckBulkPermissionsRequestItem {
	obj := rel.Object()
	subj := rel.Subject()

	return &v1.CheckBulkPermissionsRequestItem{
		Resource: &v1.ObjectReference{
			ObjectType: resourceReferenceToSpiceDBType(obj),
			ObjectId:   obj.ResourceId().Serialize(),
		},
		Permission: rel.Relation().Serialize(),
		Subject: &v1.SubjectReference{
			Object: &v1.ObjectReference{
				ObjectType: resourceReferenceToSpiceDBType(subj.Resource()),
				ObjectId:   subj.Resource().ResourceId().Serialize(),
			},
			OptionalRelation: optionalRelationToString(subj.Relation()),
		},
	}
}

func spicePairToCheckBulkResultPair(pair *v1.CheckBulkPermissionsPair, originalRequest model.Relationship) model.CheckBulkResultPair {
	if pair.GetError() != nil {
		resultItem := model.NewCheckBulkResultItem(
			false,
			fmt.Errorf("%s", pair.GetError().GetMessage()),
			pair.GetError().GetCode(),
		)
		return model.NewCheckBulkResultPair(originalRequest, resultItem)
	}

	allowed := pair.GetItem().GetPermissionship() == v1.CheckPermissionResponse_PERMISSIONSHIP_HAS_PERMISSION
	resultItem := model.NewCheckBulkResultItem(allowed, nil, 0)
	return model.NewCheckBulkResultPair(originalRequest, resultItem)
}

func relationsTupleToSpiceDBRelationship(tuple model.RelationsTuple) *v1.Relationship {
	obj := tuple.Object()
	subj := tuple.Subject()

	return &v1.Relationship{
		Resource: &v1.ObjectReference{
			ObjectType: resourceReferenceToSpiceDBType(obj),
			ObjectId:   obj.ResourceId().Serialize(),
		},
		Relation: addRelationPrefix(tuple.Relation().Serialize(), relationPrefix),
		Subject: &v1.SubjectReference{
			Object: &v1.ObjectReference{
				ObjectType: resourceReferenceToSpiceDBType(subj.Resource()),
				ObjectId:   subj.Resource().ResourceId().Serialize(),
			},
			// subject relations are intentionally not prefixed here
			// bc we want to reference the corresponding permission
			OptionalRelation: optionalRelationToString(subj.Relation()),
		},
	}
}

func tupleFilterToSpiceDBFilter(filter model.TupleFilter) (*v1.RelationshipFilter, error) {
	// Validate: ReporterType and ObjectType must be specified together
	// ReporterType serves as the namespace for resources, so both are required for proper identification
	if filter.ReporterType() != nil && filter.ObjectType() == nil {
		return nil, fmt.Errorf("if reporter type is specified then resource type must also be specified")
	}
	if filter.ReporterType() == nil && filter.ObjectType() != nil {
		return nil, fmt.Errorf("if resource type is specified then reporter type must also be specified")
	}

	var resourceType string
	if filter.ReporterType() != nil && filter.ObjectType() != nil {
		resourceType = fmt.Sprintf("%s/%s", filter.ReporterType().Serialize(), filter.ObjectType().Serialize())
	} else if filter.ObjectType() != nil {
		resourceType = filter.ObjectType().Serialize()
	}

	spiceFilter := &v1.RelationshipFilter{
		ResourceType:       resourceType,
		OptionalResourceId: optionalLocalResourceIdToString(filter.ObjectId()),
		OptionalRelation:   optionalRelationToStringWithPrefix(filter.Relation()),
	}

	if filter.Subject() != nil {
		subjectFilter := filter.Subject()

		// Validate: Subject ReporterType and SubjectType must be specified together
		// ReporterType serves as the namespace for subjects, so both are required for proper identification
		if subjectFilter.ReporterType() != nil && subjectFilter.SubjectType() == nil {
			return nil, fmt.Errorf("if subject reporter type is specified then subject type must also be specified")
		}
		if subjectFilter.ReporterType() == nil && subjectFilter.SubjectType() != nil {
			return nil, fmt.Errorf("if subject type is specified then subject reporter type must also be specified")
		}

		var subjectType string
		if subjectFilter.ReporterType() != nil && subjectFilter.SubjectType() != nil {
			subjectType = fmt.Sprintf("%s/%s", subjectFilter.ReporterType().Serialize(), subjectFilter.SubjectType().Serialize())
		} else if subjectFilter.SubjectType() != nil {
			subjectType = subjectFilter.SubjectType().Serialize()
		}

		spiceSubjectFilter := &v1.SubjectFilter{
			SubjectType:       subjectType,
			OptionalSubjectId: optionalLocalResourceIdToString(subjectFilter.SubjectId()),
		}

		// Handle subject relation filter semantics:
		// * nil: means "any relation" (wildcard), so OptionalRelation stays nil
		// * non-nil: filter for specific relation, set OptionalRelation with that value
		if subjectFilter.Relation() != nil {
			spiceSubjectFilter.OptionalRelation = &v1.SubjectFilter_RelationFilter{
				Relation: subjectFilter.Relation().Serialize(),
			}
		}

		spiceFilter.OptionalSubjectFilter = spiceSubjectFilter
	}

	return spiceFilter, nil
}

func (s *SpiceDBRelationsRepository) determineConsistency(consistency model.Consistency) *v1.Consistency {
	switch model.ConsistencyTypeOf(consistency) {
	case model.ConsistencyAtLeastAsFresh:
		fresh, _ := model.AsAtLeastAsFresh(consistency)
		return &v1.Consistency{
			Requirement: &v1.Consistency_AtLeastAsFresh{
				AtLeastAsFresh: &v1.ZedToken{Token: fresh.ConsistencyToken().Serialize()},
			},
		}
	case model.ConsistencyMinimizeLatency:
		return &v1.Consistency{
			Requirement: &v1.Consistency_MinimizeLatency{
				MinimizeLatency: true,
			},
		}
	default:
		// This flag will effectively change the default consistency behaviour
		// depending on config setting. If no consistency object is sent in a request
		// the default will either be fullyConsistent if set true or minimize_latency if false.
		if s.fullyConsistentAsDefault {
			// will ensure that all data used is fully consistent with the latest data available within the SpiceDB datastore.
			return &v1.Consistency{Requirement: &v1.Consistency_FullyConsistent{FullyConsistent: true}}
		}
		// Default consistency for read APIs is minimize_latency
		return &v1.Consistency{
			Requirement: &v1.Consistency_MinimizeLatency{
				MinimizeLatency: true,
			},
		}
	}
}

func optionalRelationToString(r *model.Relation) string {
	if r == nil {
		return ""
	}
	return r.Serialize()
}

func optionalRelationToStringWithPrefix(r *model.Relation) string {
	if r == nil {
		return ""
	}
	return addRelationPrefix(r.Serialize(), relationPrefix)
}

func optionalLocalResourceIdToString(id *model.LocalResourceId) string {
	if id == nil {
		return ""
	}
	return id.Serialize()
}

func addRelationPrefix(relation, prefix string) string {
	if !strings.HasPrefix(relation, prefix) {
		return prefix + relation
	}
	return relation
}

func stripRelationPrefix(relation, prefix string) string {
	return strings.TrimPrefix(relation, prefix)
}

func readFile(file string) (string, error) {
	bytes, err := os.ReadFile(file)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

// Stream implementations

type spicedbLookupObjectsStream struct {
	client     v1.PermissionsService_LookupResourcesClient
	objectType model.RepresentationType
}

func newSpicedbLookupObjectsStream(client v1.PermissionsService_LookupResourcesClient, objectType model.RepresentationType) *spicedbLookupObjectsStream {
	return &spicedbLookupObjectsStream{client: client, objectType: objectType}
}

func (s *spicedbLookupObjectsStream) Recv() (model.LookupObjectsItem, error) {
	msg, err := s.client.Recv()
	if err != nil {
		if errors.Is(err, io.EOF) {
			return model.LookupObjectsItem{}, io.EOF
		}
		return model.LookupObjectsItem{}, err
	}

	// LookupResourcesResponse only returns the object id; the resource type is
	// fixed by the request, so build the reference from s.objectType.
	var reporterRef *model.ReporterReference
	if rt := s.objectType.ReporterType(); rt != nil {
		r := model.NewReporterReference(*rt, nil)
		reporterRef = &r
	}
	resourceRef := model.NewResourceReference(
		s.objectType.ResourceType(),
		model.DeserializeLocalResourceId(msg.ResourceObjectId),
		reporterRef,
	)

	var continuation model.ContinuationToken
	if msg.AfterResultCursor != nil {
		continuation = model.DeserializeContinuationToken(msg.AfterResultCursor.Token)
	}

	return model.NewLookupObjectsItem(resourceRef, continuation), nil
}

type spicedbLookupSubjectsStream struct {
	client      v1.PermissionsService_LookupSubjectsClient
	subjectType model.RepresentationType
}

func newSpicedbLookupSubjectsStream(client v1.PermissionsService_LookupSubjectsClient, subjectType model.RepresentationType) *spicedbLookupSubjectsStream {
	return &spicedbLookupSubjectsStream{client: client, subjectType: subjectType}
}

func (s *spicedbLookupSubjectsStream) Recv() (model.LookupSubjectsItem, error) {
	msg, err := s.client.Recv()
	if err != nil {
		if errors.Is(err, io.EOF) {
			return model.LookupSubjectsItem{}, io.EOF
		}
		return model.LookupSubjectsItem{}, err
	}

	subj := msg.GetSubject()
	reporter := s.subjectType.ReporterType()
	subjectResource := model.NewResourceReference(
		s.subjectType.ResourceType(),
		model.DeserializeLocalResourceId(subj.SubjectObjectId),
		func() *model.ReporterReference {
			if reporter != nil {
				r := model.NewReporterReference(*reporter, nil)
				return &r
			}
			return nil
		}(),
	)

	subjectRef := model.NewSubjectReferenceWithoutRelation(subjectResource)

	var continuation model.ContinuationToken
	if msg.AfterResultCursor != nil {
		continuation = model.DeserializeContinuationToken(msg.AfterResultCursor.Token)
	}

	return model.NewLookupSubjectsItem(subjectRef, continuation), nil
}

type spicedbReadTuplesStream struct {
	client v1.PermissionsService_ReadRelationshipsClient
}

func newSpicedbReadTuplesStream(client v1.PermissionsService_ReadRelationshipsClient) *spicedbReadTuplesStream {
	return &spicedbReadTuplesStream{client: client}
}

func (s *spicedbReadTuplesStream) Recv() (model.ReadTuplesItem, error) {
	msg, err := s.client.Recv()
	if err != nil {
		if errors.Is(err, io.EOF) {
			return model.ReadTuplesItem{}, io.EOF
		}
		return model.ReadTuplesItem{}, err
	}

	spiceRel := msg.GetRelationship()

	objRef, err := spiceDBTypeToResourceReference(spiceRel.Resource.ObjectType, spiceRel.Resource.ObjectId)
	if err != nil {
		return model.ReadTuplesItem{}, fmt.Errorf("invalid resource type: %w", err)
	}

	subjResourceRef, err := spiceDBTypeToResourceReference(
		spiceRel.Subject.Object.ObjectType,
		spiceRel.Subject.Object.ObjectId,
	)
	if err != nil {
		return model.ReadTuplesItem{}, fmt.Errorf("invalid subject type: %w", err)
	}

	var subjRelation *model.Relation
	if spiceRel.Subject.OptionalRelation != "" {
		r := model.DeserializeRelation(spiceRel.Subject.OptionalRelation)
		subjRelation = &r
	}

	subjRef := model.NewSubjectReference(subjResourceRef, subjRelation)
	relation := model.DeserializeRelation(stripRelationPrefix(spiceRel.Relation, relationPrefix))

	var continuation model.ContinuationToken
	if msg.AfterResultCursor != nil {
		continuation = model.DeserializeContinuationToken(msg.AfterResultCursor.Token)
	}

	consistencyToken := model.DeserializeConsistencyToken(msg.ReadAt.GetToken())

	return model.NewReadTuplesItem(objRef, relation, subjRef, continuation, consistencyToken), nil
}
