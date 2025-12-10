package spicedb

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	authzapi "github.com/project-kessel/inventory-api/internal/authz/api"
	kessel "github.com/project-kessel/inventory-api/internal/authz/model"

	v1 "github.com/authzed/authzed-go/proto/authzed/api/v1"
	"github.com/authzed/authzed-go/v1"
	"github.com/authzed/grpcutil"
	kerrors "github.com/go-kratos/kratos/v2/errors"
	"github.com/go-kratos/kratos/v2/log"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health/grpc_health_v1"
)

// SpiceDbRepository wraps the Authzed SpiceDB client
type SpiceDbRepository struct {
	client          *authzed.Client
	healthClient    grpc_health_v1.HealthClient
	schemaFilePath  string
	isInitialized   bool
	fullyConsistent bool
	log             *log.Helper
}

const (
	relationPrefix      = "t_"
	lockType            = "kessel/lock"
	lockVersionType     = "kessel/lockversion"
	lockVersionRelation = "version"
)

// NewSpiceDbRepository creates a new SpiceDB repository
func NewSpiceDbRepository(c *Config, logger *log.Helper) (*SpiceDbRepository, error) {
	logger.Info("creating spicedb connection")

	var opts []grpc.DialOption
	opts = append(opts, grpc.EmptyDialOption{})

	var token string
	var err error
	if c.Token != "" {
		token = c.Token
	} else if c.TokenFile != "" {
		token, err = readFile(c.TokenFile)
		if err != nil {
			logger.Error(err)
			return nil, fmt.Errorf("error creating spicedb client: error loading token file: %w", err)
		}
	}
	if token == "" {
		return nil, fmt.Errorf("error creating spicedb client: token is empty")
	}

	if !c.UseTLS {
		opts = append(opts, grpcutil.WithInsecureBearerToken(token))
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	} else {
		tlsConfig, _ := grpcutil.WithSystemCerts(grpcutil.VerifyCA)
		opts = append(opts, grpcutil.WithBearerToken(token))
		opts = append(opts, tlsConfig)
	}

	client, err := authzed.NewClient(
		c.Endpoint,
		opts...,
	)

	if err != nil {
		return nil, fmt.Errorf("error creating spicedb client: %w", err)
	}

	// Create health client for readyz
	conn, err := grpc.NewClient(
		c.Endpoint,
		opts...,
	)
	if err != nil {
		return nil, fmt.Errorf("error creating grpc health client: %w", err)
	}
	healthClient := grpc_health_v1.NewHealthClient(conn)

	return &SpiceDbRepository{
		client:          client,
		healthClient:    healthClient,
		schemaFilePath:  c.SchemaFile,
		isInitialized:   false,
		fullyConsistent: c.FullyConsistent,
		log:             logger,
	}, nil
}

func (s *SpiceDbRepository) initialize() error {
	if s.isInitialized {
		return nil
	}

	schema, err := readFile(s.schemaFilePath)
	if err != nil {
		return fmt.Errorf("failed to load schema file: %w", err)
	}

	_, err = s.client.WriteSchema(context.TODO(), &v1.WriteSchemaRequest{
		Schema: schema,
	})

	if err != nil {
		return err
	}

	s.isInitialized = true
	return nil
}

func (s *SpiceDbRepository) LookupSubjects(ctx context.Context, subject_type *kessel.ObjectType, subject_relation, relation string, object *kessel.ObjectReference, limit uint32, continuation authzapi.ContinuationToken, consistency *kessel.Consistency) (chan *authzapi.SubjectResult, chan error, error) {
	if err := s.initialize(); err != nil {
		return nil, nil, err
	}

	var cursor *v1.Cursor = nil
	if continuation != "" {
		cursor = &v1.Cursor{
			Token: string(continuation),
		}
	}

	req := &v1.LookupSubjectsRequest{
		Consistency: s.determineConsistency(consistency),
		Resource: &v1.ObjectReference{
			ObjectType: kesselTypeToSpiceDBType(object.Type),
			ObjectId:   object.Id,
		},
		Permission:              relation,
		SubjectObjectType:       kesselTypeToSpiceDBType(subject_type),
		WildcardOption:          v1.LookupSubjectsRequest_WILDCARD_OPTION_EXCLUDE_WILDCARDS,
		OptionalSubjectRelation: subject_relation,
		OptionalConcreteLimit:   limit,
		OptionalCursor:          cursor,
	}

	client, err := s.client.LookupSubjects(ctx, req)

	if err != nil {
		return nil, nil, fmt.Errorf("error invoking LookupSubjects in SpiceDB: %w", err)
	}

	subjects := make(chan *authzapi.SubjectResult)
	errs := make(chan error, 1)

	go func() {
		for {
			msg, err := client.Recv()
			if err != nil {
				if !errors.Is(err, io.EOF) {
					errs <- fmt.Errorf("error receiving subject from SpiceDB: %w", err)
				}
				close(errs)
				close(subjects)
				return
			}

			continuation := authzapi.ContinuationToken("")
			if msg.AfterResultCursor != nil {
				continuation = authzapi.ContinuationToken(msg.AfterResultCursor.Token)
			}

			subj := msg.GetSubject()
			subjects <- &authzapi.SubjectResult{
				Subject: &kessel.SubjectReference{
					Subject: &kessel.ObjectReference{
						Type: subject_type,
						Id:   subj.SubjectObjectId,
					},
				},
				Continuation:     continuation,
				ConsistencyToken: &kessel.ConsistencyToken{Token: msg.GetLookedUpAt().GetToken()},
			}
		}
	}()

	return subjects, errs, nil
}

func (s *SpiceDbRepository) LookupResources(ctx context.Context, resouce_type *kessel.ObjectType, relation string, subject *kessel.SubjectReference, limit uint32, continuation authzapi.ContinuationToken, consistency *kessel.Consistency) (chan *authzapi.ResourceResult, chan error, error) {
	if err := s.initialize(); err != nil {
		return nil, nil, err
	}

	var cursor *v1.Cursor = nil
	if continuation != "" {
		cursor = &v1.Cursor{
			Token: string(continuation),
		}
	}
	client, err := s.client.LookupResources(ctx, &v1.LookupResourcesRequest{
		Consistency:        s.determineConsistency(consistency),
		ResourceObjectType: kesselTypeToSpiceDBType(resouce_type),
		Permission:         relation,
		Subject: &v1.SubjectReference{
			OptionalRelation: optionalStringPointerToString(subject.Relation),
			Object: &v1.ObjectReference{
				ObjectType: kesselTypeToSpiceDBType(subject.Subject.Type),
				ObjectId:   subject.Subject.Id,
			},
		},
		OptionalLimit:  limit,
		OptionalCursor: cursor,
	})
	if err != nil {
		return nil, nil, err
	}

	resources := make(chan *authzapi.ResourceResult)
	errs := make(chan error, 1)

	go func() {
		for {
			msg, err := client.Recv()
			if err != nil {
				if !errors.Is(err, io.EOF) {
					errs <- err
				}
				close(errs)
				close(resources)
				return
			}

			continuation := authzapi.ContinuationToken("")
			if msg.AfterResultCursor != nil {
				continuation = authzapi.ContinuationToken(msg.AfterResultCursor.Token)
			}

			resId := msg.GetResourceObjectId()
			resources <- &authzapi.ResourceResult{
				Resource: &kessel.ObjectReference{
					Type: resouce_type,
					Id:   resId,
				},
				Continuation:     continuation,
				ConsistencyToken: &kessel.ConsistencyToken{Token: msg.GetLookedUpAt().GetToken()},
			}
		}
	}()
	return resources, errs, nil
}

func (s *SpiceDbRepository) ImportBulkTuples(stream grpc.ClientStreamingServer[kessel.ImportBulkTuplesRequest, kessel.ImportBulkTuplesResponse]) error {
	if err := s.initialize(); err != nil {
		return err
	}

	var totalImported uint64
	client, err := s.client.ImportBulkRelationships(context.Background())
	if err != nil {
		return fmt.Errorf("failed to create SpiceDB client: %w", err)
	}

	for {
		req, streamErr := stream.Recv()
		if streamErr != nil {
			if req == nil && errors.Is(streamErr, io.EOF) {
				if res, closeErr := client.CloseAndRecv(); closeErr != nil {
					return fmt.Errorf("error receiving response from Spicedb for bulkimport request: %w", closeErr)
				} else {
					s.log.Infof("total number of relationships loaded: %d", res.NumLoaded)
					totalImported = res.NumLoaded
					return stream.SendAndClose(&kessel.ImportBulkTuplesResponse{NumImported: totalImported})
				}
			}
			return streamErr
		}
		inputRelationships := (*req).Tuples
		batch := []*v1.Relationship{}
		for _, tuple := range inputRelationships {
			tuple.Relation = addRelationPrefix(tuple.Relation, relationPrefix)
			batch = append(batch, createSpiceDbRelationship(tuple))
		}
		if err = client.Send((*v1.ImportBulkRelationshipsRequest)(&v1.BulkImportRelationshipsRequest{
			Relationships: batch,
		})); err != nil {
			if !errors.Is(err, io.EOF) {
				return fmt.Errorf("failed to send bulkimport request: %w", err)
			}
			return err
		}
	}

}

func (s *SpiceDbRepository) CreateRelationships(ctx context.Context, rels []*kessel.Relationship, touch authzapi.TouchSemantics, fencing *kessel.FencingCheck) (*kessel.CreateRelationshipsResponse, error) {
	if err := s.initialize(); err != nil {
		return nil, err
	}

	var relationshipUpdates []*v1.RelationshipUpdate

	var operation v1.RelationshipUpdate_Operation
	if touch {
		operation = v1.RelationshipUpdate_OPERATION_TOUCH
	} else {
		operation = v1.RelationshipUpdate_OPERATION_CREATE
	}

	for _, rel := range rels {
		rel.Relation = addRelationPrefix(rel.Relation, relationPrefix)
		// subject relations are intentionally not prefixed here
		// bc we want to reference the corresponding permission

		relationshipUpdates = append(relationshipUpdates, &v1.RelationshipUpdate{
			Operation:    operation,
			Relationship: createSpiceDbRelationship(rel),
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
					OptionalResourceId: fencing.LockId,
					OptionalRelation:   addRelationPrefix(lockVersionRelation, relationPrefix),
					OptionalSubjectFilter: &v1.SubjectFilter{
						SubjectType:       lockVersionType,
						OptionalSubjectId: fencing.LockToken,
					},
				},
			},
		}
	}

	resp, err := s.client.WriteRelationships(ctx, req)

	if err != nil {
		return nil, fmt.Errorf("error writing relationships to SpiceDB: %w", err)
	}

	return &kessel.CreateRelationshipsResponse{ConsistencyToken: &kessel.ConsistencyToken{Token: resp.GetWrittenAt().GetToken()}}, nil
}

func (s *SpiceDbRepository) ReadRelationships(ctx context.Context, filter *kessel.RelationTupleFilter, limit uint32, continuation authzapi.ContinuationToken, consistency *kessel.Consistency) (chan *authzapi.RelationshipResult, chan error, error) {
	if err := s.initialize(); err != nil {
		return nil, nil, err
	}

	var cursor *v1.Cursor = nil
	if continuation != "" {
		cursor = &v1.Cursor{
			Token: string(continuation),
		}
	}

	if filter.Relation != nil && *filter.Relation != "" {
		// subject relations are intentionally not prefixed here
		// bc we want to reference the corresponding permission
		tempRelation := addRelationPrefix(*filter.Relation, relationPrefix)
		filter.Relation = &tempRelation
	}

	relationshipFilter, err := createSpiceDbRelationshipFilter(filter)

	if err != nil {
		return nil, nil, kerrors.BadRequest("SpiceDb request validation", err.Error()).WithCause(err)
	}

	req := &v1.ReadRelationshipsRequest{
		Consistency:        s.determineConsistency(consistency),
		RelationshipFilter: relationshipFilter,
		OptionalLimit:      limit,
		OptionalCursor:     cursor,
	}

	client, err := s.client.ReadRelationships(ctx, req)

	if err != nil {
		return nil, nil, fmt.Errorf("error invoking WriteRelationships in SpiceDB: %w", err)
	}

	relationshipTuples := make(chan *authzapi.RelationshipResult)
	errs := make(chan error, 1)

	go func() {
		for {
			msg, err := client.Recv()
			if err != nil {
				if !errors.Is(err, io.EOF) {
					errs <- err
				}
				close(errs)
				close(relationshipTuples)
				return
			}

			continuation := authzapi.ContinuationToken("")
			if msg.AfterResultCursor != nil {
				continuation = authzapi.ContinuationToken(msg.AfterResultCursor.Token)
			}

			spiceDbRel := msg.GetRelationship()
			relationshipTuples <- &authzapi.RelationshipResult{
				Relationship: &kessel.Relationship{
					Resource: &kessel.ObjectReference{
						Type: spicedbTypeToKesselType(spiceDbRel.Resource.ObjectType),
						Id:   spiceDbRel.Resource.ObjectId,
					},
					Relation: strings.TrimPrefix(msg.Relationship.Relation, relationPrefix),
					Subject: &kessel.SubjectReference{
						Relation: optionalStringToStringPointer(spiceDbRel.Subject.OptionalRelation),
						Subject: &kessel.ObjectReference{
							Type: spicedbTypeToKesselType(spiceDbRel.Subject.Object.ObjectType),
							Id:   spiceDbRel.Subject.Object.ObjectId,
						},
					},
				},
				Continuation:     continuation,
				ConsistencyToken: &kessel.ConsistencyToken{Token: msg.ReadAt.GetToken()},
			}
		}
	}()

	return relationshipTuples, errs, nil
}

func (s *SpiceDbRepository) DeleteRelationships(ctx context.Context, filter *kessel.RelationTupleFilter, fencing *kessel.FencingCheck) (*kessel.DeleteRelationshipsResponse, error) {
	if err := s.initialize(); err != nil {
		return nil, err
	}

	if filter.Relation != nil && *filter.Relation != "" && filter.ResourceType != nil && *filter.ResourceType != "" {
		tempRelation := addRelationPrefix(*filter.Relation, relationPrefix)
		filter.Relation = &tempRelation
	}

	relationshipFilter, err := createSpiceDbRelationshipFilter(filter)

	if err != nil {
		return nil, kerrors.BadRequest("SpiceDb request validation", err.Error()).WithCause(err)
	}

	req := &v1.DeleteRelationshipsRequest{RelationshipFilter: relationshipFilter}

	if fencing != nil {
		req.OptionalPreconditions = []*v1.Precondition{
			{
				Operation: v1.Precondition_OPERATION_MUST_MATCH,
				Filter: &v1.RelationshipFilter{
					ResourceType:       lockType,
					OptionalResourceId: fencing.LockId,
					OptionalRelation:   addRelationPrefix(lockVersionRelation, relationPrefix),
					OptionalSubjectFilter: &v1.SubjectFilter{
						SubjectType:       lockVersionType,
						OptionalSubjectId: fencing.LockToken,
					},
				},
			},
		}
	}

	resp, err := s.client.DeleteRelationships(ctx, req)

	if err != nil {
		return nil, fmt.Errorf("error invoking DeleteRelationships in SpiceDB %w", err)
	}

	return &kessel.DeleteRelationshipsResponse{ConsistencyToken: &kessel.ConsistencyToken{Token: resp.GetDeletedAt().GetToken()}}, nil
}

func (s *SpiceDbRepository) Check(ctx context.Context, check *kessel.CheckRequest) (*kessel.CheckResponse, error) {
	if err := s.initialize(); err != nil {
		return nil, err
	}

	subject := &v1.SubjectReference{
		Object: &v1.ObjectReference{
			ObjectType: kesselTypeToSpiceDBType(check.Subject.Subject.Type),
			ObjectId:   check.Subject.Subject.Id,
		},
		OptionalRelation: optionalStringPointerToString(check.Subject.Relation),
	}

	resource := &v1.ObjectReference{
		ObjectType: kesselTypeToSpiceDBType(check.Resource.Type),
		ObjectId:   check.Resource.Id,
	}
	req := &v1.CheckPermissionRequest{
		Consistency: s.determineConsistency(check.Consistency),
		Resource:    resource,
		Permission:  check.Relation,
		Subject:     subject,
	}
	checkResponse, err := s.client.CheckPermission(ctx, req)
	if err != nil {
		return &kessel.CheckResponse{Allowed: kessel.AllowedUnspecified}, fmt.Errorf("error invoking CheckPermission in SpiceDB: %w", err)
	}

	if checkResponse.Permissionship == v1.CheckPermissionResponse_PERMISSIONSHIP_HAS_PERMISSION {
		return &kessel.CheckResponse{
			Allowed:          kessel.AllowedTrue,
			ConsistencyToken: &kessel.ConsistencyToken{Token: checkResponse.GetCheckedAt().GetToken()},
		}, nil
	}

	return &kessel.CheckResponse{
		Allowed:          kessel.AllowedFalse,
		ConsistencyToken: &kessel.ConsistencyToken{Token: checkResponse.GetCheckedAt().GetToken()},
	}, nil
}

func (s *SpiceDbRepository) CheckForUpdate(ctx context.Context, check *kessel.CheckForUpdateRequest) (*kessel.CheckForUpdateResponse, error) {
	if err := s.initialize(); err != nil {
		return nil, err
	}

	subject := &v1.SubjectReference{
		Object: &v1.ObjectReference{
			ObjectType: kesselTypeToSpiceDBType(check.Subject.Subject.Type),
			ObjectId:   check.Subject.Subject.Id,
		},
		OptionalRelation: optionalStringPointerToString(check.Subject.Relation),
	}

	resource := &v1.ObjectReference{
		ObjectType: kesselTypeToSpiceDBType(check.Resource.Type),
		ObjectId:   check.Resource.Id,
	}
	req := &v1.CheckPermissionRequest{
		Consistency: &v1.Consistency{Requirement: &v1.Consistency_FullyConsistent{FullyConsistent: true}},
		Resource:    resource,
		Permission:  check.Relation,
		Subject:     subject,
	}
	checkResponse, err := s.client.CheckPermission(ctx, req)
	if err != nil {
		return &kessel.CheckForUpdateResponse{Allowed: kessel.AllowedUnspecified}, fmt.Errorf("error invoking CheckPermission in SpiceDB: %w", err)
	}

	if checkResponse.Permissionship == v1.CheckPermissionResponse_PERMISSIONSHIP_HAS_PERMISSION {
		return &kessel.CheckForUpdateResponse{
			Allowed:          kessel.AllowedTrue,
			ConsistencyToken: &kessel.ConsistencyToken{Token: checkResponse.GetCheckedAt().GetToken()},
		}, nil
	}

	return &kessel.CheckForUpdateResponse{
		Allowed:          kessel.AllowedFalse,
		ConsistencyToken: &kessel.ConsistencyToken{Token: checkResponse.GetCheckedAt().GetToken()},
	}, nil
}

// helper to build a SpiceDB CheckBulkPermissionsRequestItem from your API type
func toSpiceItem(item *kessel.CheckBulkRequestItem) *v1.CheckBulkPermissionsRequestItem {
	return &v1.CheckBulkPermissionsRequestItem{
		Resource: &v1.ObjectReference{
			ObjectType: kesselTypeToSpiceDBType(item.Resource.Type),
			ObjectId:   item.Resource.Id,
		},
		Permission: item.Relation,
		Subject: &v1.SubjectReference{
			Object: &v1.ObjectReference{
				ObjectType: kesselTypeToSpiceDBType(item.Subject.Subject.Type),
				ObjectId:   item.Subject.Subject.Id,
			},
			OptionalRelation: optionalStringPointerToString(item.Subject.Relation),
		},
	}
}

// helper to convert a SpiceDB pair to your API type
func fromSpicePair(pair *v1.CheckBulkPermissionsPair, log *log.Helper) *kessel.CheckBulkResponsePair {
	req := pair.GetRequest()
	request := &kessel.CheckBulkRequestItem{
		Resource: &kessel.ObjectReference{
			Type: spicedbTypeToKesselType(req.GetResource().GetObjectType()),
			Id:   req.GetResource().GetObjectId(),
		},
		Relation: req.GetPermission(),
		Subject: &kessel.SubjectReference{
			Subject: &kessel.ObjectReference{
				Type: spicedbTypeToKesselType(req.GetSubject().GetObject().GetObjectType()),
				Id:   req.GetSubject().GetObject().GetObjectId(),
			},
			Relation: optionalStringToStringPointer(req.GetSubject().GetOptionalRelation()),
		},
	}

	if pair.GetError() != nil {
		log.Errorf("Error in checkbulk for req: %v error-code: %v error-message: %v", request, pair.GetError().GetCode(), pair.GetError().GetMessage())
		return &kessel.CheckBulkResponsePair{
			Request: request,
			Error:   fmt.Errorf("%s", pair.GetError().GetMessage()),
		}

	}

	allowed := kessel.AllowedFalse
	if pair.GetItem().GetPermissionship() == v1.CheckPermissionResponse_PERMISSIONSHIP_HAS_PERMISSION {
		allowed = kessel.AllowedTrue
	}
	return &kessel.CheckBulkResponsePair{
		Request: request,
		Item:    &kessel.CheckBulkResponseItem{Allowed: allowed},
	}
}

func (s *SpiceDbRepository) CheckBulk(ctx context.Context, check *kessel.CheckBulkRequest) (*kessel.CheckBulkResponse, error) {

	if err := s.initialize(); err != nil {
		return nil, err
	}
	items := make([]*v1.CheckBulkPermissionsRequestItem, len(check.Items))
	for i, it := range check.Items {
		items[i] = toSpiceItem(it)
	}
	req := &v1.CheckBulkPermissionsRequest{Consistency: s.determineConsistency(check.Consistency), Items: items}

	resp, err := s.client.CheckBulkPermissions(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("error invoking CheckBulkPermissions in SpiceDB: %w", err)
	}

	pairs := make([]*kessel.CheckBulkResponsePair, len(resp.Pairs))
	for i, p := range resp.Pairs {
		pairs[i] = fromSpicePair(p, s.log)
	}
	return &kessel.CheckBulkResponse{
		Pairs:            pairs,
		ConsistencyToken: &kessel.ConsistencyToken{Token: resp.GetCheckedAt().GetToken()},
	}, nil
}

func (s *SpiceDbRepository) IsBackendAvailable() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := s.healthClient.Check(ctx, &grpc_health_v1.HealthCheckRequest{})
	if err != nil {
		return err
	}

	select {
	case <-ctx.Done():
		return fmt.Errorf("timeout connecting to backend")
	default:
		switch resp.Status {
		case grpc_health_v1.HealthCheckResponse_NOT_SERVING, grpc_health_v1.HealthCheckResponse_SERVICE_UNKNOWN:
			return fmt.Errorf("error connecting to backend: %v", resp.Status.String())
		case grpc_health_v1.HealthCheckResponse_SERVING:
			return nil
		}
	}
	return fmt.Errorf("error connecting to backend")
}

// Health implements the Health interface for compatibility with inventory-api
func (s *SpiceDbRepository) Health(ctx context.Context) (*kessel.GetReadyzResponse, error) {
	err := s.IsBackendAvailable()
	if err != nil {
		return &kessel.GetReadyzResponse{Status: "not ready", Code: 1}, nil
	}
	return &kessel.GetReadyzResponse{Status: "ready", Code: 0}, nil
}

func (s *SpiceDbRepository) AcquireLock(ctx context.Context, lockId string) (*kessel.AcquireLockResponse, error) {
	if err := s.initialize(); err != nil {
		return nil, err
	}

	newFencingToken := uuid.New().String()
	readClient, err := s.client.ReadRelationships(ctx, &v1.ReadRelationshipsRequest{
		Consistency: &v1.Consistency{Requirement: &v1.Consistency_FullyConsistent{FullyConsistent: true}},
		RelationshipFilter: &v1.RelationshipFilter{
			ResourceType:       lockType,
			OptionalResourceId: lockId,
			OptionalRelation:   addRelationPrefix(lockVersionRelation, relationPrefix),
		},
	})
	if err != nil {
		return nil, fmt.Errorf("error invoking ReadRelationships in SpiceDB: %w", err)
	}

	existingLock, err := readClient.Recv()
	if err != nil && !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("error reading existing lock: %w", err)
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
				ObjectId:   lockId,
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
		return nil, fmt.Errorf("error writing relationships to SpiceDB: %w", err)
	}

	return &kessel.AcquireLockResponse{LockToken: newFencingToken}, nil
}

func createSpiceDbRelationshipFilter(filter *kessel.RelationTupleFilter) (*v1.RelationshipFilter, error) {
	// spicedb specific internal validation to reflect spicedb limitations whereby namespace and objectType must be both
	// be set if either of them is set in a filter
	resourceNamespace := ""
	if filter.ResourceNamespace != nil {
		resourceNamespace = *filter.ResourceNamespace
	}
	resourceType := ""
	if filter.ResourceType != nil {
		resourceType = *filter.ResourceType
	}

	if resourceNamespace != "" && resourceType == "" {
		return nil, fmt.Errorf("due to a spicedb limitation, if resource namespace is specified then resource type must also be specified")
	}
	if resourceNamespace == "" && resourceType != "" {
		return nil, fmt.Errorf("due to a spicedb limitation, if resource type is specified then resource namespace must also be specified")
	}

	resourceTypeObj := &kessel.ObjectType{Namespace: resourceNamespace, Name: resourceType}

	resourceId := ""
	if filter.ResourceId != nil {
		resourceId = *filter.ResourceId
	}
	relation := ""
	if filter.Relation != nil {
		relation = *filter.Relation
	}

	spiceDbRelationshipFilter := &v1.RelationshipFilter{
		ResourceType:       kesselTypeToSpiceDBType(resourceTypeObj),
		OptionalResourceId: resourceId,
		OptionalRelation:   relation,
	}

	if filter.SubjectFilter != nil {
		subjectFilter := filter.SubjectFilter

		subjectNamespace := ""
		if subjectFilter.SubjectNamespace != nil {
			subjectNamespace = *subjectFilter.SubjectNamespace
		}
		subjectType := ""
		if subjectFilter.SubjectType != nil {
			subjectType = *subjectFilter.SubjectType
		}

		if subjectNamespace != "" && subjectType == "" {
			return nil, fmt.Errorf("due to a spicedb limitation, if subject namespace is specified in subjectFilter then subject type must also be specified")
		}
		if subjectNamespace == "" && subjectType != "" {
			return nil, fmt.Errorf("due to a spicedb limitation, if subject type is specified in subjectFilter then subject namespace must also be specified")
		}

		subjectTypeObj := &kessel.ObjectType{Namespace: subjectNamespace, Name: subjectType}

		subjectId := ""
		if subjectFilter.SubjectId != nil {
			subjectId = *subjectFilter.SubjectId
		}

		spiceDbSubjectFilter := &v1.SubjectFilter{
			SubjectType:       kesselTypeToSpiceDBType(subjectTypeObj),
			OptionalSubjectId: subjectId,
		}

		if subjectFilter.Relation != nil && *subjectFilter.Relation != "" {
			spiceDbSubjectFilter.OptionalRelation = &v1.SubjectFilter_RelationFilter{
				Relation: *subjectFilter.Relation,
			}
		}

		spiceDbRelationshipFilter.OptionalSubjectFilter = spiceDbSubjectFilter
	}

	return spiceDbRelationshipFilter, nil
}

func spicedbTypeToKesselType(spicedbType string) *kessel.ObjectType {
	kesselType := &kessel.ObjectType{}

	parts := strings.Split(spicedbType, "/")
	switch len(parts) {
	case 1:
		kesselType.Name = parts[0]
	case 2:
		kesselType.Namespace = parts[0]
		kesselType.Name = parts[1]
	default:
		return nil //?? Error?
	}

	return kesselType
}

func kesselTypeToSpiceDBType(kesselType *kessel.ObjectType) string {
	if kesselType.Namespace != "" {
		return fmt.Sprintf("%s/%s", kesselType.Namespace, kesselType.Name)
	}

	return kesselType.Name
}

func optionalStringPointerToString(optional *string) string {
	if optional == nil {
		return ""
	}
	return *optional
}

func optionalStringToStringPointer(optional string) *string {
	if optional == "" {
		return nil
	}

	return &optional
}

func addRelationPrefix(relation, prefix string) string {
	if !strings.HasPrefix(relation, prefix) {
		return prefix + relation
	}
	return relation
}

func createSpiceDbRelationship(relationship *kessel.Relationship) *v1.Relationship {
	subject := &v1.SubjectReference{
		Object: &v1.ObjectReference{
			ObjectType: kesselTypeToSpiceDBType(relationship.Subject.Subject.Type),
			ObjectId:   relationship.Subject.Subject.Id,
		},
		OptionalRelation: optionalStringPointerToString(relationship.Subject.Relation),
	}

	object := &v1.ObjectReference{
		ObjectType: kesselTypeToSpiceDBType(relationship.Resource.Type),
		ObjectId:   relationship.Resource.Id,
	}

	return &v1.Relationship{
		Resource: object,
		Relation: relationship.Relation,
		Subject:  subject,
	}
}

func readFile(file string) (string, error) {
	bytes, err := os.ReadFile(file)
	if err != nil {
		return "", err
	}

	return string(bytes), nil
}

func (s *SpiceDbRepository) determineConsistency(consistency *kessel.Consistency) *v1.Consistency {
	if consistency != nil && consistency.AtLeastAsFresh != nil {
		return &v1.Consistency{
			Requirement: &v1.Consistency_AtLeastAsFresh{
				AtLeastAsFresh: &v1.ZedToken{Token: consistency.AtLeastAsFresh.Token},
			},
		}
	}

	if consistency != nil && consistency.MinimizeLatency {
		return &v1.Consistency{
			Requirement: &v1.Consistency_MinimizeLatency{
				MinimizeLatency: true,
			},
		}
	}

	// This flag will effectively change the default consistency behaviour
	// depending on config setting. If no consistency object is sent in a request
	// the default will either be fullyConsistent if set true or minimize_latency if false.
	if s.fullyConsistent {
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
