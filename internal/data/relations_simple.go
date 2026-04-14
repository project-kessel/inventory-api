package data

import (
	"context"
	"io"
	"maps"
	"slices"
	"strconv"
	"sync"

	kesselv1 "github.com/project-kessel/relations-api/api/kessel/relations/v1"
	kessel "github.com/project-kessel/relations-api/api/kessel/relations/v1beta1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

// simpleTupleKey represents a unique relationship tuple for lookup.
// This mirrors the structure of kessel.Relationship but as a comparable key.
type simpleTupleKey struct {
	ResourceNamespace string
	ResourceType      string
	ResourceID        string
	Relation          string
	SubjectNamespace  string
	SubjectType       string
	SubjectID         string
}

// SimpleRelationsRepository implements RelationsRepository with a simple tuple-based model for testing.
// It stores relationship tuples via CreateTuples and checks them via Check methods.
// This is not a full ReBAC implementation - it only supports direct tuple lookups,
// not computed relations or permission expansion.
//
// # Snapshot Support
//
// SimpleRelationsRepository maintains a version counter that increments on every mutation.
// By default, only the latest state is kept (fully consistent reads).
// Tests can retain old snapshots via RetainCurrentSnapshot() to test consistency
// token behavior. Check operations with an "at least as fresh" token will use
// the oldest retained snapshot that is >= the requested version.
type SimpleRelationsRepository struct {
	mu        sync.RWMutex
	version   int64                             // current version (monotonically increasing)
	tuples    map[simpleTupleKey]bool           // current/latest tuple state
	snapshots map[int64]map[simpleTupleKey]bool // retained historical snapshots (version -> tuples)
}

// NewSimpleRelationsRepository creates a SimpleRelationsRepository with no tuples at version 1.
func NewSimpleRelationsRepository() *SimpleRelationsRepository {
	return &SimpleRelationsRepository{
		version:   1,
		tuples:    make(map[simpleTupleKey]bool),
		snapshots: make(map[int64]map[simpleTupleKey]bool),
	}
}

// Version returns the current version number.
func (s *SimpleRelationsRepository) Version() int64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.version
}

// RetainCurrentSnapshot saves the current tuple state as a retained snapshot.
// This allows tests to verify consistency token behavior by making changes
// after retaining a snapshot, then checking with the old token.
func (s *SimpleRelationsRepository) RetainCurrentSnapshot() int64 {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Copy current tuples to snapshot
	snapshot := make(map[simpleTupleKey]bool, len(s.tuples))
	maps.Copy(snapshot, s.tuples)
	s.snapshots[s.version] = snapshot
	return s.version
}

// ReleaseSnapshot removes a retained snapshot, allowing it to be garbage collected.
func (s *SimpleRelationsRepository) ReleaseSnapshot(version int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.snapshots, version)
}

// ClearSnapshots removes all retained snapshots.
func (s *SimpleRelationsRepository) ClearSnapshots() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.snapshots = make(map[int64]map[simpleTupleKey]bool)
}

// advanceVersion increments the version counter. Must be called with lock held.
func (s *SimpleRelationsRepository) advanceVersion() {
	s.version++
}

// getTuplesForToken returns the appropriate tuple map for the given consistency token.
// Returns the oldest available snapshot with version >= the requested token.
// Available snapshots include retained snapshots and the current state.
// When no token is provided (empty string) or token is invalid, treats it as 0,
// which means "use the oldest available snapshot".
// When no snapshots are retained, the current state is the only available one.
func (s *SimpleRelationsRepository) getTuplesForToken(token string) map[simpleTupleKey]bool {
	// Parse token, default to 0 (oldest available)
	var requested int64 = 0
	if token != "" {
		if parsed, err := simpleParseConsistencyToken(token); err == nil {
			requested = parsed
		}
	}

	// Collect all available versions (snapshots + current)
	versions := make([]int64, 0, len(s.snapshots)+1)
	for v := range s.snapshots {
		versions = append(versions, v)
	}
	versions = append(versions, s.version)
	slices.Sort(versions)

	// Find the minimum version >= requested
	idx, _ := slices.BinarySearch(versions, requested)
	if idx < len(versions) {
		v := versions[idx]
		if v == s.version {
			return s.tuples
		}
		return s.snapshots[v]
	}

	return s.tuples
}

func simpleFormatConsistencyToken(version int64) string {
	return strconv.FormatInt(version, 10)
}

func simpleParseConsistencyToken(token string) (int64, error) {
	return strconv.ParseInt(token, 10, 64)
}

// Grant is a convenience method for tests to add a direct permission tuple.
// It creates a tuple: (namespace/resourceType:resourceID)#relation@(rbac/principal:subjectID)
// This advances the version counter.
func (s *SimpleRelationsRepository) Grant(subjectID, relation, namespace, resourceType, resourceID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tuples[simpleTupleKey{
		ResourceNamespace: namespace,
		ResourceType:      resourceType,
		ResourceID:        resourceID,
		Relation:          relation,
		SubjectNamespace:  "rbac",
		SubjectType:       "principal",
		SubjectID:         subjectID,
	}] = true
	s.advanceVersion()
}

// Reset clears all tuples and snapshots, resetting version to 1.
func (s *SimpleRelationsRepository) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tuples = make(map[simpleTupleKey]bool)
	s.snapshots = make(map[int64]map[simpleTupleKey]bool)
	s.version = 1
}

func simpleHasTupleInSnapshot(tuples map[simpleTupleKey]bool, resourceNamespace, resourceType, resourceID, relation, subjectNamespace, subjectType, subjectID string) bool {
	key := simpleTupleKey{
		ResourceNamespace: resourceNamespace,
		ResourceType:      resourceType,
		ResourceID:        resourceID,
		Relation:          relation,
		SubjectNamespace:  subjectNamespace,
		SubjectType:       subjectType,
		SubjectID:         subjectID,
	}
	return tuples[key]
}

func simpleTupleKeyFromRelationship(rel *kessel.Relationship) simpleTupleKey {
	key := simpleTupleKey{}
	if rel.Resource != nil {
		key.ResourceID = rel.Resource.Id
		if rel.Resource.Type != nil {
			key.ResourceNamespace = rel.Resource.Type.Namespace
			key.ResourceType = rel.Resource.Type.Name
		}
	}
	key.Relation = rel.Relation
	if rel.Subject != nil && rel.Subject.Subject != nil {
		key.SubjectID = rel.Subject.Subject.Id
		if rel.Subject.Subject.Type != nil {
			key.SubjectNamespace = rel.Subject.Subject.Type.Namespace
			key.SubjectType = rel.Subject.Subject.Type.Name
		}
	}
	return key
}

func (s *SimpleRelationsRepository) Health(_ context.Context) (*kesselv1.GetReadyzResponse, error) {
	return &kesselv1.GetReadyzResponse{}, nil
}

func (s *SimpleRelationsRepository) Check(_ context.Context, namespace, permission, consistencyToken, resourceType, localResourceID string, sub *kessel.SubjectReference) (kessel.CheckResponse_Allowed, *kessel.ConsistencyToken, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	subjectNamespace := ""
	subjectType := ""
	subjectID := ""
	if sub != nil && sub.Subject != nil {
		subjectID = sub.Subject.Id
		if sub.Subject.Type != nil {
			subjectNamespace = sub.Subject.Type.Namespace
			subjectType = sub.Subject.Type.Name
		}
	}

	tuples := s.getTuplesForToken(consistencyToken)
	resultToken := simpleFormatConsistencyToken(s.version)

	if simpleHasTupleInSnapshot(tuples, namespace, resourceType, localResourceID, permission, subjectNamespace, subjectType, subjectID) {
		return kessel.CheckResponse_ALLOWED_TRUE, &kessel.ConsistencyToken{Token: resultToken}, nil
	}
	return kessel.CheckResponse_ALLOWED_FALSE, &kessel.ConsistencyToken{Token: resultToken}, nil
}

func (s *SimpleRelationsRepository) CheckForUpdate(_ context.Context, namespace, permission, resourceType, localResourceID string, sub *kessel.SubjectReference) (kessel.CheckForUpdateResponse_Allowed, *kessel.ConsistencyToken, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	subjectNamespace := ""
	subjectType := ""
	subjectID := ""
	if sub != nil && sub.Subject != nil {
		subjectID = sub.Subject.Id
		if sub.Subject.Type != nil {
			subjectNamespace = sub.Subject.Type.Namespace
			subjectType = sub.Subject.Type.Name
		}
	}

	resultToken := simpleFormatConsistencyToken(s.version)

	if simpleHasTupleInSnapshot(s.tuples, namespace, resourceType, localResourceID, permission, subjectNamespace, subjectType, subjectID) {
		return kessel.CheckForUpdateResponse_ALLOWED_TRUE, &kessel.ConsistencyToken{Token: resultToken}, nil
	}
	return kessel.CheckForUpdateResponse_ALLOWED_FALSE, &kessel.ConsistencyToken{Token: resultToken}, nil
}

func (s *SimpleRelationsRepository) CheckBulk(_ context.Context, req *kessel.CheckBulkRequest) (*kessel.CheckBulkResponse, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	consistencyToken := ""
	if req.Consistency != nil {
		if atLeastAsFresh := req.Consistency.GetAtLeastAsFresh(); atLeastAsFresh != nil {
			consistencyToken = atLeastAsFresh.GetToken()
		}
	}

	tuples := s.getTuplesForToken(consistencyToken)
	resultToken := simpleFormatConsistencyToken(s.version)

	pairs := make([]*kessel.CheckBulkResponsePair, len(req.GetItems()))
	for i, item := range req.GetItems() {
		subjectNamespace := ""
		subjectType := ""
		subjectID := ""
		if item.Subject != nil && item.Subject.Subject != nil {
			subjectID = item.Subject.Subject.Id
			if item.Subject.Subject.Type != nil {
				subjectNamespace = item.Subject.Subject.Type.Namespace
				subjectType = item.Subject.Subject.Type.Name
			}
		}

		resourceNamespace := ""
		resourceType := ""
		resourceID := ""
		if item.Resource != nil {
			resourceID = item.Resource.Id
			if item.Resource.Type != nil {
				resourceNamespace = item.Resource.Type.Namespace
				resourceType = item.Resource.Type.Name
			}
		}

		allowed := kessel.CheckBulkResponseItem_ALLOWED_FALSE
		if simpleHasTupleInSnapshot(tuples, resourceNamespace, resourceType, resourceID, item.Relation, subjectNamespace, subjectType, subjectID) {
			allowed = kessel.CheckBulkResponseItem_ALLOWED_TRUE
		}

		pairs[i] = &kessel.CheckBulkResponsePair{
			Request: item,
			Response: &kessel.CheckBulkResponsePair_Item{
				Item: &kessel.CheckBulkResponseItem{
					Allowed: allowed,
				},
			},
		}
	}

	return &kessel.CheckBulkResponse{
		Pairs:            pairs,
		ConsistencyToken: &kessel.ConsistencyToken{Token: resultToken},
	}, nil
}

func (s *SimpleRelationsRepository) CheckForUpdateBulk(ctx context.Context, req *kessel.CheckForUpdateBulkRequest) (*kessel.CheckForUpdateBulkResponse, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	tuples := s.getTuplesForToken("")
	resultToken := simpleFormatConsistencyToken(s.version)

	pairs := make([]*kessel.CheckBulkResponsePair, len(req.GetItems()))
	for i, item := range req.GetItems() {
		subjectNamespace := ""
		subjectType := ""
		subjectID := ""
		if item.Subject != nil && item.Subject.Subject != nil {
			subjectID = item.Subject.Subject.Id
			if item.Subject.Subject.Type != nil {
				subjectNamespace = item.Subject.Subject.Type.Namespace
				subjectType = item.Subject.Subject.Type.Name
			}
		}
		resourceNamespace := ""
		resourceType := ""
		resourceID := ""
		if item.Resource != nil {
			resourceID = item.Resource.Id
			if item.Resource.Type != nil {
				resourceNamespace = item.Resource.Type.Namespace
				resourceType = item.Resource.Type.Name
			}
		}
		allowed := kessel.CheckBulkResponseItem_ALLOWED_FALSE
		if simpleHasTupleInSnapshot(tuples, resourceNamespace, resourceType, resourceID, item.Relation, subjectNamespace, subjectType, subjectID) {
			allowed = kessel.CheckBulkResponseItem_ALLOWED_TRUE
		}
		pairs[i] = &kessel.CheckBulkResponsePair{
			Request: item,
			Response: &kessel.CheckBulkResponsePair_Item{
				Item: &kessel.CheckBulkResponseItem{Allowed: allowed},
			},
		}
	}
	return &kessel.CheckForUpdateBulkResponse{
		Pairs:            pairs,
		ConsistencyToken: &kessel.ConsistencyToken{Token: resultToken},
	}, nil
}

func (s *SimpleRelationsRepository) LookupResources(_ context.Context, req *kessel.LookupResourcesRequest) (grpc.ServerStreamingClient[kessel.LookupResourcesResponse], error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	requestedNamespace := ""
	requestedType := ""
	if req.ResourceType != nil {
		requestedNamespace = req.ResourceType.Namespace
		requestedType = req.ResourceType.Name
	}
	requestedRelation := req.Relation

	subjectNamespace := ""
	subjectType := ""
	subjectID := ""
	if req.Subject != nil && req.Subject.Subject != nil {
		subjectID = req.Subject.Subject.Id
		if req.Subject.Subject.Type != nil {
			subjectNamespace = req.Subject.Subject.Type.Namespace
			subjectType = req.Subject.Subject.Type.Name
		}
	}

	var results []*kessel.LookupResourcesResponse
	for key := range s.tuples {
		namespaceMatches := requestedNamespace == "" || key.ResourceNamespace == requestedNamespace
		typeMatches := requestedType == "" || key.ResourceType == requestedType
		relationMatches := key.Relation == requestedRelation
		subjectMatches := key.SubjectNamespace == subjectNamespace &&
			key.SubjectType == subjectType &&
			key.SubjectID == subjectID

		if namespaceMatches && typeMatches && relationMatches && subjectMatches {
			results = append(results, &kessel.LookupResourcesResponse{
				Resource: &kessel.ObjectReference{
					Type: &kessel.ObjectType{
						Namespace: key.ResourceNamespace,
						Name:      key.ResourceType,
					},
					Id: key.ResourceID,
				},
				Pagination: &kessel.ResponsePagination{},
			})
		}
	}

	return &simpleLookupResourcesStream{results: results}, nil
}

func (s *SimpleRelationsRepository) LookupSubjects(_ context.Context, req *kessel.LookupSubjectsRequest) (grpc.ServerStreamingClient[kessel.LookupSubjectsResponse], error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	resourceNamespace := ""
	resourceType := ""
	resourceID := ""
	if req.Resource != nil {
		resourceID = req.Resource.Id
		if req.Resource.Type != nil {
			resourceNamespace = req.Resource.Type.Namespace
			resourceType = req.Resource.Type.Name
		}
	}
	requestedRelation := req.Relation

	subjectNamespace := ""
	subjectType := ""
	if req.SubjectType != nil {
		subjectNamespace = req.SubjectType.Namespace
		subjectType = req.SubjectType.Name
	}

	var results []*kessel.LookupSubjectsResponse
	for key := range s.tuples {
		resourceMatches := key.ResourceNamespace == resourceNamespace &&
			key.ResourceType == resourceType &&
			key.ResourceID == resourceID
		relationMatches := key.Relation == requestedRelation
		subjectTypeMatches := (subjectNamespace == "" || key.SubjectNamespace == subjectNamespace) &&
			(subjectType == "" || key.SubjectType == subjectType)

		if resourceMatches && relationMatches && subjectTypeMatches {
			results = append(results, &kessel.LookupSubjectsResponse{
				Subject: &kessel.SubjectReference{
					Subject: &kessel.ObjectReference{
						Type: &kessel.ObjectType{
							Namespace: key.SubjectNamespace,
							Name:      key.SubjectType,
						},
						Id: key.SubjectID,
					},
				},
				Pagination: &kessel.ResponsePagination{},
			})
		}
	}

	return &simpleLookupSubjectsStream{results: results}, nil
}

func (s *SimpleRelationsRepository) CreateTuples(_ context.Context, req *kessel.CreateTuplesRequest) (*kessel.CreateTuplesResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, rel := range req.GetTuples() {
		key := simpleTupleKeyFromRelationship(rel)
		s.tuples[key] = true
	}
	s.advanceVersion()

	return &kessel.CreateTuplesResponse{}, nil
}

func (s *SimpleRelationsRepository) DeleteTuples(_ context.Context, req *kessel.DeleteTuplesRequest) (*kessel.DeleteTuplesResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	filter := req.GetFilter()
	if filter == nil {
		s.advanceVersion()
		return &kessel.DeleteTuplesResponse{}, nil
	}

	for key := range s.tuples {
		if simpleMatchesFilter(key, filter) {
			delete(s.tuples, key)
		}
	}
	s.advanceVersion()

	return &kessel.DeleteTuplesResponse{}, nil
}

func simpleMatchesFilter(key simpleTupleKey, filter *kessel.RelationTupleFilter) bool {
	if filter.ResourceNamespace != nil && *filter.ResourceNamespace != key.ResourceNamespace {
		return false
	}
	if filter.ResourceType != nil && *filter.ResourceType != key.ResourceType {
		return false
	}
	if filter.ResourceId != nil && *filter.ResourceId != key.ResourceID {
		return false
	}
	if filter.Relation != nil && *filter.Relation != key.Relation {
		return false
	}
	if filter.SubjectFilter != nil {
		sf := filter.SubjectFilter
		if sf.SubjectNamespace != nil && *sf.SubjectNamespace != key.SubjectNamespace {
			return false
		}
		if sf.SubjectType != nil && *sf.SubjectType != key.SubjectType {
			return false
		}
		if sf.SubjectId != nil && *sf.SubjectId != key.SubjectID {
			return false
		}
	}
	return true
}

func (s *SimpleRelationsRepository) AcquireLock(_ context.Context, _ *kessel.AcquireLockRequest) (*kessel.AcquireLockResponse, error) {
	return &kessel.AcquireLockResponse{}, nil
}

func (s *SimpleRelationsRepository) UnsetWorkspace(_ context.Context, _, _, _ string) (*kessel.DeleteTuplesResponse, error) {
	return &kessel.DeleteTuplesResponse{}, nil
}

func (s *SimpleRelationsRepository) SetWorkspace(_ context.Context, _, _, _, _ string, _ bool) (*kessel.CreateTuplesResponse, error) {
	return &kessel.CreateTuplesResponse{}, nil
}

type simpleLookupResourcesStream struct {
	results []*kessel.LookupResourcesResponse
	index   int
}

func (s *simpleLookupResourcesStream) Recv() (*kessel.LookupResourcesResponse, error) {
	if s.index >= len(s.results) {
		return nil, io.EOF
	}
	result := s.results[s.index]
	s.index++
	return result, nil
}

func (s *simpleLookupResourcesStream) Header() (metadata.MD, error) {
	return nil, nil
}

func (s *simpleLookupResourcesStream) Trailer() metadata.MD {
	return nil
}

func (s *simpleLookupResourcesStream) CloseSend() error {
	return nil
}

func (s *simpleLookupResourcesStream) Context() context.Context {
	return context.Background()
}

func (s *simpleLookupResourcesStream) SendMsg(_ interface{}) error {
	return nil
}

func (s *simpleLookupResourcesStream) RecvMsg(_ interface{}) error {
	return nil
}

type simpleLookupSubjectsStream struct {
	results []*kessel.LookupSubjectsResponse
	index   int
}

func (s *simpleLookupSubjectsStream) Recv() (*kessel.LookupSubjectsResponse, error) {
	if s.index >= len(s.results) {
		return nil, io.EOF
	}
	result := s.results[s.index]
	s.index++
	return result, nil
}

func (s *simpleLookupSubjectsStream) Header() (metadata.MD, error) {
	return nil, nil
}

func (s *simpleLookupSubjectsStream) Trailer() metadata.MD {
	return nil
}

func (s *simpleLookupSubjectsStream) CloseSend() error {
	return nil
}

func (s *simpleLookupSubjectsStream) Context() context.Context {
	return context.Background()
}

func (s *simpleLookupSubjectsStream) SendMsg(_ interface{}) error {
	return nil
}

func (s *simpleLookupSubjectsStream) RecvMsg(_ interface{}) error {
	return nil
}
