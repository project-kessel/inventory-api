package data

import (
	"context"
	"io"
	"maps"
	"slices"
	"strconv"
	"sync"

	"github.com/project-kessel/inventory-api/internal/biz/model"
)

// simpleTupleKey represents a unique relationship tuple for lookup.
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
//
// # Failure Simulation
//
// Tests can configure failure modes via SetHealthError(), SetCreateTuplesError(),
// SetDeleteTuplesError(), and SetAcquireLockError() to simulate Relations API failures.
type SimpleRelationsRepository struct {
	mu                sync.RWMutex
	version           int64                             // current version (monotonically increasing)
	tuples            map[simpleTupleKey]bool           // current/latest tuple state
	snapshots         map[int64]map[simpleTupleKey]bool // retained historical snapshots (version -> tuples)
	healthError       error                             // if set, Health() returns this error
	createTuplesError error                             // if set, CreateTuples() returns this error
	deleteTuplesError error                             // if set, DeleteTuples() returns this error
	acquireLockError  error                             // if set, AcquireLock() returns this error
	locks             map[string]string                 // lockId -> token
}

var _ model.RelationsRepository = &SimpleRelationsRepository{}

// NewSimpleRelationsRepository creates a SimpleRelationsRepository with no tuples at version 1.
func NewSimpleRelationsRepository() *SimpleRelationsRepository {
	return &SimpleRelationsRepository{
		version:   1,
		tuples:    make(map[simpleTupleKey]bool),
		snapshots: make(map[int64]map[simpleTupleKey]bool),
		locks:     make(map[string]string),
	}
}

// Version returns the current version number.
func (s *SimpleRelationsRepository) Version() int64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.version
}

// RetainCurrentSnapshot saves the current tuple state as a retained snapshot.
func (s *SimpleRelationsRepository) RetainCurrentSnapshot() int64 {
	s.mu.Lock()
	defer s.mu.Unlock()

	snapshot := make(map[simpleTupleKey]bool, len(s.tuples))
	maps.Copy(snapshot, s.tuples)
	s.snapshots[s.version] = snapshot
	return s.version
}

// ReleaseSnapshot removes a retained snapshot.
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

func (s *SimpleRelationsRepository) advanceVersion() {
	s.version++
}

func (s *SimpleRelationsRepository) getTuplesForToken(token string) map[simpleTupleKey]bool {
	var requested int64 = 0
	if token != "" {
		if parsed, err := simpleParseConsistencyToken(token); err == nil {
			requested = parsed
		}
	}

	versions := make([]int64, 0, len(s.snapshots)+1)
	for v := range s.snapshots {
		versions = append(versions, v)
	}
	versions = append(versions, s.version)
	slices.Sort(versions)

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

// Reset restores the repository to its initial state.
func (s *SimpleRelationsRepository) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tuples = make(map[simpleTupleKey]bool)
	s.snapshots = make(map[int64]map[simpleTupleKey]bool)
	s.locks = make(map[string]string)
	s.version = 1
	s.healthError = nil
	s.createTuplesError = nil
	s.deleteTuplesError = nil
	s.acquireLockError = nil
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

func simpleTupleKeyFromModelTuple(tuple model.RelationsTuple) simpleTupleKey {
	obj := tuple.Object()
	sub := tuple.Subject().Resource()
	key := simpleTupleKey{
		ResourceType: obj.ResourceType().Serialize(),
		ResourceID:   obj.ResourceId().Serialize(),
		Relation:     tuple.Relation().Serialize(),
		SubjectType:  sub.ResourceType().Serialize(),
		SubjectID:    sub.ResourceId().Serialize(),
	}
	if obj.HasReporter() {
		key.ResourceNamespace = obj.Reporter().ReporterType().Serialize()
	}
	if sub.HasReporter() {
		key.SubjectNamespace = sub.Reporter().ReporterType().Serialize()
	}
	return key
}

// SetHealthError configures the error returned by Health().
func (s *SimpleRelationsRepository) SetHealthError(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.healthError = err
}

// SetCreateTuplesError configures the error that CreateTuples() will return.
func (s *SimpleRelationsRepository) SetCreateTuplesError(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.createTuplesError = err
}

// SetDeleteTuplesError configures the error that DeleteTuples() will return.
func (s *SimpleRelationsRepository) SetDeleteTuplesError(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.deleteTuplesError = err
}

// SetAcquireLockError configures the error that AcquireLock() will return.
func (s *SimpleRelationsRepository) SetAcquireLockError(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.acquireLockError = err
}

func (s *SimpleRelationsRepository) Health(_ context.Context) (model.HealthResult, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.healthError != nil {
		return model.HealthResult{}, s.healthError
	}
	return model.NewHealthResult("OK", 200), nil
}

func simpleResourceRefFields(ref model.ResourceReference) (namespace, resType, resID string) {
	resType = ref.ResourceType().Serialize()
	resID = ref.ResourceId().Serialize()
	if ref.HasReporter() {
		namespace = ref.Reporter().ReporterType().Serialize()
	}
	return
}

func (s *SimpleRelationsRepository) Check(_ context.Context, rel model.Relationship, consistency model.Consistency,
) (model.CheckResult, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	consistencyToken := consistencyToSimpleToken(consistency)
	tuples := s.getTuplesForToken(consistencyToken)
	resultToken := model.DeserializeConsistencyToken(simpleFormatConsistencyToken(s.version))

	objNs, objType, objId := simpleResourceRefFields(rel.Object())
	subResource := rel.Subject().Resource()
	subNs, subType, subId := simpleResourceRefFields(subResource)

	allowed := simpleHasTupleInSnapshot(tuples, objNs, objType, objId,
		rel.Relation().Serialize(), subNs, subType, subId)

	return model.NewCheckResult(allowed, resultToken), nil
}

func (s *SimpleRelationsRepository) CheckForUpdate(_ context.Context, rel model.Relationship,
) (model.CheckResult, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	resultToken := model.DeserializeConsistencyToken(simpleFormatConsistencyToken(s.version))

	objNs, objType, objId := simpleResourceRefFields(rel.Object())
	subResource := rel.Subject().Resource()
	subNs, subType, subId := simpleResourceRefFields(subResource)

	allowed := simpleHasTupleInSnapshot(s.tuples, objNs, objType, objId,
		rel.Relation().Serialize(), subNs, subType, subId)

	return model.NewCheckResult(allowed, resultToken), nil
}

func (s *SimpleRelationsRepository) CheckBulk(_ context.Context, rels []model.Relationship, consistency model.Consistency,
) (model.CheckBulkResult, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	consistencyToken := consistencyToSimpleToken(consistency)
	tuples := s.getTuplesForToken(consistencyToken)
	resultToken := model.DeserializeConsistencyToken(simpleFormatConsistencyToken(s.version))

	pairs := make([]model.CheckBulkResultPair, len(rels))
	for i, rel := range rels {
		objNs, objType, objId := simpleResourceRefFields(rel.Object())
		subResource := rel.Subject().Resource()
		subNs, subType, subId := simpleResourceRefFields(subResource)

		allowed := simpleHasTupleInSnapshot(tuples, objNs, objType, objId,
			rel.Relation().Serialize(), subNs, subType, subId)

		pairs[i] = model.NewCheckBulkResultPair(rel, model.NewCheckBulkResultItem(allowed, nil, 0))
	}

	return model.NewCheckBulkResult(pairs, resultToken), nil
}

func (s *SimpleRelationsRepository) CheckForUpdateBulk(_ context.Context, rels []model.Relationship,
) (model.CheckBulkResult, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	resultToken := model.DeserializeConsistencyToken(simpleFormatConsistencyToken(s.version))

	pairs := make([]model.CheckBulkResultPair, len(rels))
	for i, rel := range rels {
		objNs, objType, objId := simpleResourceRefFields(rel.Object())
		subResource := rel.Subject().Resource()
		subNs, subType, subId := simpleResourceRefFields(subResource)

		allowed := simpleHasTupleInSnapshot(s.tuples, objNs, objType, objId,
			rel.Relation().Serialize(), subNs, subType, subId)

		pairs[i] = model.NewCheckBulkResultPair(rel, model.NewCheckBulkResultItem(allowed, nil, 0))
	}

	return model.NewCheckBulkResult(pairs, resultToken), nil
}

func (s *SimpleRelationsRepository) LookupObjects(_ context.Context,
	objectType model.RepresentationType,
	relation model.Relation, subject model.SubjectReference,
	_ *model.Pagination, _ model.Consistency,
) (model.ResultStream[model.LookupObjectsItem], error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	requestedType := objectType.ResourceType().Serialize()
	requestedNamespace := ""
	if objectType.HasReporterType() {
		requestedNamespace = objectType.ReporterType().Serialize()
	}
	requestedRelation := relation.Serialize()

	subResource := subject.Resource()
	subNs, subType, subId := simpleResourceRefFields(subResource)

	var results []model.LookupObjectsItem
	for key := range s.tuples {
		namespaceMatches := requestedNamespace == "" || key.ResourceNamespace == requestedNamespace
		typeMatches := requestedType == "" || key.ResourceType == requestedType
		relationMatches := key.Relation == requestedRelation
		subjectMatches := key.SubjectNamespace == subNs &&
			key.SubjectType == subType &&
			key.SubjectID == subId

		if namespaceMatches && typeMatches && relationMatches && subjectMatches {
			reporterType := model.DeserializeReporterType(key.ResourceNamespace)
			reporter := model.NewReporterReference(reporterType, nil)
			results = append(results, model.NewLookupObjectsItem(
				model.NewResourceReference(
					model.DeserializeResourceType(key.ResourceType),
					model.DeserializeLocalResourceId(key.ResourceID),
					&reporter,
				), "",
			))
		}
	}

	return &simpleLookupObjectsStream{results: results}, nil
}

func (s *SimpleRelationsRepository) LookupSubjects(_ context.Context,
	object model.ResourceReference, relation model.Relation,
	subjectType model.RepresentationType,
	_ *model.Relation,
	_ *model.Pagination, _ model.Consistency,
) (model.ResultStream[model.LookupSubjectsItem], error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	objNs, objType, objId := simpleResourceRefFields(object)
	requestedRelation := relation.Serialize()

	wantSubjectType := subjectType.ResourceType().Serialize()
	wantSubjectNamespace := ""
	if subjectType.HasReporterType() {
		wantSubjectNamespace = subjectType.ReporterType().Serialize()
	}

	var results []model.LookupSubjectsItem
	for key := range s.tuples {
		resourceMatches := key.ResourceNamespace == objNs &&
			key.ResourceType == objType &&
			key.ResourceID == objId
		relationMatches := key.Relation == requestedRelation
		subjectTypeMatches := (wantSubjectNamespace == "" || key.SubjectNamespace == wantSubjectNamespace) &&
			(wantSubjectType == "" || key.SubjectType == wantSubjectType)

		if resourceMatches && relationMatches && subjectTypeMatches {
			reporterType := model.DeserializeReporterType(key.SubjectNamespace)
			reporter := model.NewReporterReference(reporterType, nil)
			subResource := model.NewResourceReference(
				model.DeserializeResourceType(key.SubjectType),
				model.DeserializeLocalResourceId(key.SubjectID),
				&reporter,
			)
			results = append(results, model.NewLookupSubjectsItem(
				model.NewSubjectReferenceWithoutRelation(subResource), "",
			))
		}
	}

	return &simpleLookupSubjectsStream{results: results}, nil
}

func (s *SimpleRelationsRepository) CreateTuples(_ context.Context, tuples []model.RelationsTuple, _ bool, _ *model.FencingCheck,
) (model.TuplesResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.createTuplesError != nil {
		return model.TuplesResult{}, s.createTuplesError
	}

	for _, tuple := range tuples {
		key := simpleTupleKeyFromModelTuple(tuple)
		s.tuples[key] = true
	}
	s.advanceVersion()

	return model.NewTuplesResult(model.DeserializeConsistencyToken(strconv.FormatInt(s.version, 10))), nil
}

func (s *SimpleRelationsRepository) DeleteTuples(_ context.Context, filter model.TupleFilter, _ *model.FencingCheck,
) (model.TuplesResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.deleteTuplesError != nil {
		return model.TuplesResult{}, s.deleteTuplesError
	}

	for key := range s.tuples {
		if simpleMatchesTupleFilter(key, filter) {
			delete(s.tuples, key)
		}
	}
	s.advanceVersion()

	return model.NewTuplesResult(model.DeserializeConsistencyToken(strconv.FormatInt(s.version, 10))), nil
}

func simpleMatchesTupleFilter(key simpleTupleKey, filter model.TupleFilter) bool {
	if filter.ReporterType() != nil && filter.ReporterType().Serialize() != key.ResourceNamespace {
		return false
	}
	if filter.ObjectType() != nil && filter.ObjectType().Serialize() != key.ResourceType {
		return false
	}
	if filter.ObjectId() != nil && filter.ObjectId().Serialize() != key.ResourceID {
		return false
	}
	if filter.Relation() != nil && filter.Relation().Serialize() != key.Relation {
		return false
	}
	if filter.Subject() != nil {
		sf := filter.Subject()
		if sf.ReporterType() != nil && sf.ReporterType().Serialize() != key.SubjectNamespace {
			return false
		}
		if sf.SubjectType() != nil && sf.SubjectType().Serialize() != key.SubjectType {
			return false
		}
		if sf.SubjectId() != nil && sf.SubjectId().Serialize() != key.SubjectID {
			return false
		}
	}
	return true
}

func (s *SimpleRelationsRepository) ReadTuples(_ context.Context, filter model.TupleFilter, _ *model.Pagination, _ model.Consistency,
) (model.ResultStream[model.ReadTuplesItem], error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var results []model.ReadTuplesItem
	for key := range s.tuples {
		if simpleMatchesTupleFilter(key, filter) {
			objReporterType := model.DeserializeReporterType(key.ResourceNamespace)
			objReporter := model.NewReporterReference(objReporterType, nil)
			object := model.NewResourceReference(
				model.DeserializeResourceType(key.ResourceType),
				model.DeserializeLocalResourceId(key.ResourceID),
				&objReporter,
			)

			subReporterType := model.DeserializeReporterType(key.SubjectNamespace)
			subReporter := model.NewReporterReference(subReporterType, nil)
			subResource := model.NewResourceReference(
				model.DeserializeResourceType(key.SubjectType),
				model.DeserializeLocalResourceId(key.SubjectID),
				&subReporter,
			)

			results = append(results, model.NewReadTuplesItem(
				object,
				model.DeserializeRelation(key.Relation),
				model.NewSubjectReferenceWithoutRelation(subResource),
				"",
				model.MinimizeLatencyToken,
			))
		}
	}

	return &simpleReadTuplesStream{results: results}, nil
}

func (s *SimpleRelationsRepository) AcquireLock(_ context.Context, lockId model.LockId) (model.AcquireLockResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.acquireLockError != nil {
		return model.AcquireLockResult{}, s.acquireLockError
	}

	token := "token-" + lockId.String()
	s.locks[lockId.String()] = token

	return model.NewAcquireLockResult(model.DeserializeLockToken(token)), nil
}

func consistencyToSimpleToken(c model.Consistency) string {
	if token := model.ConsistencyAtLeastAsFreshToken(c); token != nil {
		return token.Serialize()
	}
	return ""
}

type simpleLookupObjectsStream struct {
	results []model.LookupObjectsItem
	index   int
}

func (s *simpleLookupObjectsStream) Recv() (model.LookupObjectsItem, error) {
	if s.index >= len(s.results) {
		return model.LookupObjectsItem{}, io.EOF
	}
	result := s.results[s.index]
	s.index++
	return result, nil
}

type simpleLookupSubjectsStream struct {
	results []model.LookupSubjectsItem
	index   int
}

func (s *simpleLookupSubjectsStream) Recv() (model.LookupSubjectsItem, error) {
	if s.index >= len(s.results) {
		return model.LookupSubjectsItem{}, io.EOF
	}
	result := s.results[s.index]
	s.index++
	return result, nil
}

type simpleReadTuplesStream struct {
	results []model.ReadTuplesItem
	index   int
}

func (s *simpleReadTuplesStream) Recv() (model.ReadTuplesItem, error) {
	if s.index >= len(s.results) {
		return model.ReadTuplesItem{}, io.EOF
	}
	result := s.results[s.index]
	s.index++
	return result, nil
}
