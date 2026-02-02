package authz

import (
	"context"
	"io"
	"maps"
	"slices"
	"strconv"
	"sync"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	kesselv1 "github.com/project-kessel/relations-api/api/kessel/relations/v1"
	kessel "github.com/project-kessel/relations-api/api/kessel/relations/v1beta1"
)

// tupleKey represents a unique relationship tuple for lookup.
// This mirrors the structure of kessel.Relationship but as a comparable key.
type tupleKey struct {
	ResourceNamespace string
	ResourceType      string
	ResourceID        string
	Relation          string
	SubjectNamespace  string
	SubjectType       string
	SubjectID         string
}

// SimpleAuthorizer implements Authorizer with a simple tuple-based model for testing.
// It stores relationship tuples via CreateTuples and checks them via Check methods.
// This is not a full ReBAC implementation - it only supports direct tuple lookups,
// not computed relations or permission expansion.
//
// # Snapshot Support
//
// SimpleAuthorizer maintains a version counter that increments on every mutation.
// By default, only the latest state is kept (fully consistent reads).
// Tests can retain old snapshots via RetainCurrentSnapshot() to test consistency
// token behavior. Check operations with an "at least as fresh" token will use
// the oldest retained snapshot that is >= the requested version.
type SimpleAuthorizer struct {
	mu        sync.RWMutex
	version   int64                       // current version (monotonically increasing)
	tuples    map[tupleKey]bool           // current/latest tuple state
	snapshots map[int64]map[tupleKey]bool // retained historical snapshots (version -> tuples)
}

// NewSimpleAuthorizer creates a SimpleAuthorizer with no tuples at version 1.
func NewSimpleAuthorizer() *SimpleAuthorizer {
	return &SimpleAuthorizer{
		version:   1,
		tuples:    make(map[tupleKey]bool),
		snapshots: make(map[int64]map[tupleKey]bool),
	}
}

// Version returns the current version number.
func (s *SimpleAuthorizer) Version() int64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.version
}

// RetainCurrentSnapshot saves the current tuple state as a retained snapshot.
// This allows tests to verify consistency token behavior by making changes
// after retaining a snapshot, then checking with the old token.
func (s *SimpleAuthorizer) RetainCurrentSnapshot() int64 {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Copy current tuples to snapshot
	snapshot := make(map[tupleKey]bool, len(s.tuples))
	maps.Copy(snapshot, s.tuples)
	s.snapshots[s.version] = snapshot
	return s.version
}

// ReleaseSnapshot removes a retained snapshot, allowing it to be garbage collected.
func (s *SimpleAuthorizer) ReleaseSnapshot(version int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.snapshots, version)
}

// ClearSnapshots removes all retained snapshots.
func (s *SimpleAuthorizer) ClearSnapshots() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.snapshots = make(map[int64]map[tupleKey]bool)
}

// advanceVersion increments the version counter. Must be called with lock held.
func (s *SimpleAuthorizer) advanceVersion() {
	s.version++
}

// getTuplesForToken returns the appropriate tuple map for the given consistency token.
// Returns the oldest available snapshot with version >= the requested token.
// Available snapshots include retained snapshots and the current state.
// When no token is provided (empty string) or token is invalid, treats it as 0,
// which means "use the oldest available snapshot".
// When no snapshots are retained, the current state is the only available one.
func (s *SimpleAuthorizer) getTuplesForToken(token string) map[tupleKey]bool {
	// Parse token, default to 0 (oldest available)
	var requested int64 = 0
	if token != "" {
		if parsed, err := parseConsistencyToken(token); err == nil {
			requested = parsed
		}
		// On parse error, use 0 (oldest available)
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

// formatConsistencyToken formats a version as a consistency token string.
func formatConsistencyToken(version int64) string {
	return strconv.FormatInt(version, 10)
}

// parseConsistencyToken parses a consistency token string into a version number.
func parseConsistencyToken(token string) (int64, error) {
	return strconv.ParseInt(token, 10, 64)
}

// Grant is a convenience method for tests to add a direct permission tuple.
// It creates a tuple: (namespace/resourceType:resourceID)#relation@(rbac/principal:subjectID)
// This advances the version counter.
func (s *SimpleAuthorizer) Grant(subjectID, relation, namespace, resourceType, resourceID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tuples[tupleKey{
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
func (s *SimpleAuthorizer) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tuples = make(map[tupleKey]bool)
	s.snapshots = make(map[int64]map[tupleKey]bool)
	s.version = 1
}

// hasTupleInSnapshot checks if a tuple exists in the given tuple map.
func hasTupleInSnapshot(tuples map[tupleKey]bool, resourceNamespace, resourceType, resourceID, relation, subjectNamespace, subjectType, subjectID string) bool {
	key := tupleKey{
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

func tupleKeyFromRelationship(rel *kessel.Relationship) tupleKey {
	key := tupleKey{}
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

// Health implements Authorizer.
func (s *SimpleAuthorizer) Health(_ context.Context) (*kesselv1.GetReadyzResponse, error) {
	return &kesselv1.GetReadyzResponse{}, nil
}

// Check implements Authorizer.
// The consistencyToken parameter (third argument) specifies the minimum freshness required.
func (s *SimpleAuthorizer) Check(_ context.Context, namespace, permission, consistencyToken, resourceType, localResourceID string, sub *kessel.SubjectReference) (kessel.CheckResponse_Allowed, *kessel.ConsistencyToken, error) {
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
	resultToken := formatConsistencyToken(s.version)

	if hasTupleInSnapshot(tuples, namespace, resourceType, localResourceID, permission, subjectNamespace, subjectType, subjectID) {
		return kessel.CheckResponse_ALLOWED_TRUE, &kessel.ConsistencyToken{Token: resultToken}, nil
	}
	return kessel.CheckResponse_ALLOWED_FALSE, &kessel.ConsistencyToken{Token: resultToken}, nil
}

// CheckForUpdate implements Authorizer.
// CheckForUpdate always uses the latest state (no stale reads for update checks).
func (s *SimpleAuthorizer) CheckForUpdate(_ context.Context, namespace, permission, resourceType, localResourceID string, sub *kessel.SubjectReference) (kessel.CheckForUpdateResponse_Allowed, *kessel.ConsistencyToken, error) {
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

	resultToken := formatConsistencyToken(s.version)

	if hasTupleInSnapshot(s.tuples, namespace, resourceType, localResourceID, permission, subjectNamespace, subjectType, subjectID) {
		return kessel.CheckForUpdateResponse_ALLOWED_TRUE, &kessel.ConsistencyToken{Token: resultToken}, nil
	}
	return kessel.CheckForUpdateResponse_ALLOWED_FALSE, &kessel.ConsistencyToken{Token: resultToken}, nil
}

// CheckBulk implements Authorizer.
// Respects the consistency token in the request if provided.
func (s *SimpleAuthorizer) CheckBulk(_ context.Context, req *kessel.CheckBulkRequest) (*kessel.CheckBulkResponse, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Extract consistency token from request
	consistencyToken := ""
	if req.Consistency != nil {
		if atLeastAsFresh := req.Consistency.GetAtLeastAsFresh(); atLeastAsFresh != nil {
			consistencyToken = atLeastAsFresh.GetToken()
		}
	}

	tuples := s.getTuplesForToken(consistencyToken)
	resultToken := formatConsistencyToken(s.version)

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
		if hasTupleInSnapshot(tuples, resourceNamespace, resourceType, resourceID, item.Relation, subjectNamespace, subjectType, subjectID) {
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

// LookupResources implements Authorizer.
// It returns resources where the subject has the specified relation.
// This is a simple direct-tuple lookup, not a full ReBAC graph traversal.
func (s *SimpleAuthorizer) LookupResources(_ context.Context, req *kessel.LookupResourcesRequest) (grpc.ServerStreamingClient[kessel.LookupResourcesResponse], error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Extract the request parameters
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

	// Find all matching tuples
	var results []*kessel.LookupResourcesResponse
	for key := range s.tuples {
		// Match tuples where:
		// - Resource namespace matches (if specified)
		// - Resource type matches (if specified)
		// - Relation matches
		// - Subject matches exactly
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

// CreateTuples implements Authorizer by storing the relationship tuples.
// This advances the version counter.
func (s *SimpleAuthorizer) CreateTuples(_ context.Context, req *kessel.CreateTuplesRequest) (*kessel.CreateTuplesResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, rel := range req.GetTuples() {
		key := tupleKeyFromRelationship(rel)
		s.tuples[key] = true
	}
	s.advanceVersion()

	return &kessel.CreateTuplesResponse{}, nil
}

// DeleteTuples implements Authorizer by removing tuples matching the filter.
// This is a simplified implementation that requires exact matches on all filter fields.
// This advances the version counter.
func (s *SimpleAuthorizer) DeleteTuples(_ context.Context, req *kessel.DeleteTuplesRequest) (*kessel.DeleteTuplesResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	filter := req.GetFilter()
	if filter == nil {
		s.advanceVersion()
		return &kessel.DeleteTuplesResponse{}, nil
	}

	// Find and delete matching tuples
	for key := range s.tuples {
		if matchesFilter(key, filter) {
			delete(s.tuples, key)
		}
	}
	s.advanceVersion()

	return &kessel.DeleteTuplesResponse{}, nil
}

func matchesFilter(key tupleKey, filter *kessel.RelationTupleFilter) bool {
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
	// Note: SubjectFilter matching is more complex, simplified here
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

// AcquireLock implements Authorizer.
func (s *SimpleAuthorizer) AcquireLock(_ context.Context, _ *kessel.AcquireLockRequest) (*kessel.AcquireLockResponse, error) {
	return &kessel.AcquireLockResponse{}, nil
}

// UnsetWorkspace implements Authorizer.
func (s *SimpleAuthorizer) UnsetWorkspace(_ context.Context, _, _, _ string) (*kessel.DeleteTuplesResponse, error) {
	return &kessel.DeleteTuplesResponse{}, nil
}

// SetWorkspace implements Authorizer.
func (s *SimpleAuthorizer) SetWorkspace(_ context.Context, _, _, _, _ string, _ bool) (*kessel.CreateTuplesResponse, error) {
	return &kessel.CreateTuplesResponse{}, nil
}

// simpleLookupResourcesStream implements grpc.ServerStreamingClient for LookupResourcesResponse.
// It returns pre-computed results in order, then returns io.EOF.
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
