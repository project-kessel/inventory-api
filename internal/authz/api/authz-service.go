package api

import (
	"context"

	"google.golang.org/grpc"

	kessel "github.com/project-kessel/inventory-api/internal/authz/model"
)

// Type definitions for streaming results (from relations-api/internal/biz)
type TouchSemantics bool
type ContinuationToken string

type RelationshipResult struct {
	Relationship     *kessel.Relationship
	Continuation     ContinuationToken
	ConsistencyToken *kessel.ConsistencyToken
}

type ResourceResult struct {
	Resource         *kessel.ObjectReference
	Continuation     ContinuationToken
	ConsistencyToken *kessel.ConsistencyToken
}

type SubjectResult struct {
	Subject          *kessel.SubjectReference
	Continuation     ContinuationToken
	ConsistencyToken *kessel.ConsistencyToken
}

// Authorizer defines the interface for authorization and access control operations.
// It provides methods for checking permissions, managing relationships, and health checks.
// This interface merges concepts from the original Authorizer and ZanzibarRepository.
type Authorizer interface {
	// Health and Availability
	Health(ctx context.Context) (*kessel.GetReadyzResponse, error)
	IsBackendAvailable() error

	// Permission Checks (using request objects)
	Check(ctx context.Context, request *kessel.CheckRequest) (*kessel.CheckResponse, error)
	CheckForUpdate(ctx context.Context, request *kessel.CheckForUpdateRequest) (*kessel.CheckForUpdateResponse, error)
	CheckBulk(ctx context.Context, request *kessel.CheckBulkRequest) (*kessel.CheckBulkResponse, error)

	// Relationship Management
	CreateRelationships(ctx context.Context, rels []*kessel.Relationship, touch TouchSemantics, fencing *kessel.FencingCheck) (*kessel.CreateRelationshipsResponse, error)
	DeleteRelationships(ctx context.Context, filter *kessel.RelationTupleFilter, fencing *kessel.FencingCheck) (*kessel.DeleteRelationshipsResponse, error)
	ReadRelationships(ctx context.Context, filter *kessel.RelationTupleFilter, limit uint32, continuation ContinuationToken, consistency *kessel.Consistency) (chan *RelationshipResult, chan error, error)

	// Lookup Operations (channels instead of gRPC streams)
	LookupResources(ctx context.Context, resourceType *kessel.ObjectType, relation string, subject *kessel.SubjectReference, limit uint32, continuation ContinuationToken, consistency *kessel.Consistency) (chan *ResourceResult, chan error, error)
	LookupSubjects(ctx context.Context, subjectType *kessel.ObjectType, subjectRelation, relation string, resource *kessel.ObjectReference, limit uint32, continuation ContinuationToken, consistency *kessel.Consistency) (chan *SubjectResult, chan error, error)

	// Bulk Import
	ImportBulkTuples(stream grpc.ClientStreamingServer[kessel.ImportBulkTuplesRequest, kessel.ImportBulkTuplesResponse]) error

	// Lock Management
	AcquireLock(ctx context.Context, lockId string) (*kessel.AcquireLockResponse, error)
}
