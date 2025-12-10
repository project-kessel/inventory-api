package allow

import (
	"context"

	"google.golang.org/grpc"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/project-kessel/inventory-api/internal/authz/api"
	kessel "github.com/project-kessel/inventory-api/internal/authz/model"
)

type AllowAllAuthz struct {
	Logger *log.Helper
}

func New(logger *log.Helper) *AllowAllAuthz {
	logger.Info("Using authorizer: allow-all")
	return &AllowAllAuthz{
		Logger: logger,
	}
}

func (a *AllowAllAuthz) Health(ctx context.Context) (*kessel.GetReadyzResponse, error) {
	return &kessel.GetReadyzResponse{Status: "OK", Code: 200}, nil
}

func (a *AllowAllAuthz) IsBackendAvailable() error {
	return nil
}

func (a *AllowAllAuthz) Check(ctx context.Context, request *kessel.CheckRequest) (*kessel.CheckResponse, error) {
	return &kessel.CheckResponse{Allowed: kessel.AllowedTrue}, nil
}

func (a *AllowAllAuthz) CheckForUpdate(ctx context.Context, request *kessel.CheckForUpdateRequest) (*kessel.CheckForUpdateResponse, error) {
	return &kessel.CheckForUpdateResponse{Allowed: kessel.AllowedTrue}, nil
}

func (a *AllowAllAuthz) CheckBulk(ctx context.Context, req *kessel.CheckBulkRequest) (*kessel.CheckBulkResponse, error) {
	pairs := make([]*kessel.CheckBulkResponsePair, len(req.Items))
	for i, item := range req.Items {
		pairs[i] = &kessel.CheckBulkResponsePair{
			Request: item,
			Item: &kessel.CheckBulkResponseItem{
				Allowed: kessel.AllowedTrue,
			},
		}
	}
	return &kessel.CheckBulkResponse{Pairs: pairs}, nil
}

func (a *AllowAllAuthz) CreateRelationships(ctx context.Context, rels []*kessel.Relationship, touch api.TouchSemantics, fencing *kessel.FencingCheck) (*kessel.CreateRelationshipsResponse, error) {
	return &kessel.CreateRelationshipsResponse{}, nil
}

func (a *AllowAllAuthz) DeleteRelationships(ctx context.Context, filter *kessel.RelationTupleFilter, fencing *kessel.FencingCheck) (*kessel.DeleteRelationshipsResponse, error) {
	return &kessel.DeleteRelationshipsResponse{}, nil
}

func (a *AllowAllAuthz) ReadRelationships(ctx context.Context, filter *kessel.RelationTupleFilter, limit uint32, continuation api.ContinuationToken, consistency *kessel.Consistency) (chan *api.RelationshipResult, chan error, error) {
	results := make(chan *api.RelationshipResult)
	errs := make(chan error, 1)
	close(results)
	close(errs)
	return results, errs, nil
}

func (a *AllowAllAuthz) LookupResources(ctx context.Context, resourceType *kessel.ObjectType, relation string, subject *kessel.SubjectReference, limit uint32, continuation api.ContinuationToken, consistency *kessel.Consistency) (chan *api.ResourceResult, chan error, error) {
	results := make(chan *api.ResourceResult)
	errs := make(chan error, 1)
	close(results)
	close(errs)
	return results, errs, nil
}

func (a *AllowAllAuthz) LookupSubjects(ctx context.Context, subjectType *kessel.ObjectType, subjectRelation, relation string, resource *kessel.ObjectReference, limit uint32, continuation api.ContinuationToken, consistency *kessel.Consistency) (chan *api.SubjectResult, chan error, error) {
	results := make(chan *api.SubjectResult)
	errs := make(chan error, 1)
	close(results)
	close(errs)
	return results, errs, nil
}

func (a *AllowAllAuthz) ImportBulkTuples(stream grpc.ClientStreamingServer[kessel.ImportBulkTuplesRequest, kessel.ImportBulkTuplesResponse]) error {
	return stream.SendAndClose(&kessel.ImportBulkTuplesResponse{NumImported: 0})
}

func (a *AllowAllAuthz) AcquireLock(ctx context.Context, lockId string) (*kessel.AcquireLockResponse, error) {
	return &kessel.AcquireLockResponse{LockToken: "fake-lock-token"}, nil
}
