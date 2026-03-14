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

// relationsTupleKey represents a unique relationship tuple for lookup.
type relationsTupleKey struct {
	ResourceNamespace string
	ResourceType      string
	ResourceID        string
	Relation          string
	SubjectNamespace  string
	SubjectType       string
	SubjectID         string
}

// FakeRelationsRepository implements RelationsRepository with a simple tuple-based
// model for testing. It stores relationship tuples via CreateTuples and checks them
// via Check methods. This is not a full ReBAC implementation -- it only supports
// direct tuple lookups, not computed relations or permission expansion.
//
// # Snapshot Support
//
// FakeRelationsRepository maintains a version counter that increments on every mutation.
// By default, only the latest state is kept (fully consistent reads).
// Tests can retain old snapshots via RetainCurrentSnapshot() to test consistency
// token behavior. Check operations with an "at least as fresh" token will use
// the oldest retained snapshot that is >= the requested version.
type FakeRelationsRepository struct {
	mu        sync.RWMutex
	version   int64
	tuples    map[relationsTupleKey]bool
	snapshots map[int64]map[relationsTupleKey]bool
}

var _ model.RelationsRepository = &FakeRelationsRepository{}

// NewFakeRelationsRepository creates a new FakeRelationsRepository for use
// as a model.RelationsRepository. Returns the concrete type so callers
// can use test helper methods.
func NewFakeRelationsRepository() *FakeRelationsRepository {
	return &FakeRelationsRepository{
		version:   1,
		tuples:    make(map[relationsTupleKey]bool),
		snapshots: make(map[int64]map[relationsTupleKey]bool),
	}
}

// Version returns the current version number.
func (f *FakeRelationsRepository) Version() int64 {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.version
}

// RetainCurrentSnapshot saves the current tuple state as a retained snapshot.
func (f *FakeRelationsRepository) RetainCurrentSnapshot() int64 {
	f.mu.Lock()
	defer f.mu.Unlock()
	snapshot := make(map[relationsTupleKey]bool, len(f.tuples))
	maps.Copy(snapshot, f.tuples)
	f.snapshots[f.version] = snapshot
	return f.version
}

// ReleaseSnapshot removes a retained snapshot.
func (f *FakeRelationsRepository) ReleaseSnapshot(version int64) {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.snapshots, version)
}

// ClearSnapshots removes all retained snapshots.
func (f *FakeRelationsRepository) ClearSnapshots() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.snapshots = make(map[int64]map[relationsTupleKey]bool)
}

// Grant is a convenience method for tests to add a direct permission tuple.
// Creates: (namespace/resourceType:resourceID)#relation@(rbac/principal:subjectID)
func (f *FakeRelationsRepository) Grant(subjectID, relation, namespace, resourceType, resourceID string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.tuples[relationsTupleKey{
		ResourceNamespace: namespace,
		ResourceType:      resourceType,
		ResourceID:        resourceID,
		Relation:          relation,
		SubjectNamespace:  "rbac",
		SubjectType:       "principal",
		SubjectID:         subjectID,
	}] = true
	f.version++
}

// Reset clears all tuples and snapshots, resetting version to 1.
func (f *FakeRelationsRepository) Reset() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.tuples = make(map[relationsTupleKey]bool)
	f.snapshots = make(map[int64]map[relationsTupleKey]bool)
	f.version = 1
}

func (f *FakeRelationsRepository) advanceVersion() {
	f.version++
}

func (f *FakeRelationsRepository) getTuplesForToken(token string) map[relationsTupleKey]bool {
	var requested int64 = 0
	if token != "" {
		if parsed, err := strconv.ParseInt(token, 10, 64); err == nil {
			requested = parsed
		}
	}

	versions := make([]int64, 0, len(f.snapshots)+1)
	for v := range f.snapshots {
		versions = append(versions, v)
	}
	versions = append(versions, f.version)
	slices.Sort(versions)

	idx, _ := slices.BinarySearch(versions, requested)
	if idx < len(versions) {
		v := versions[idx]
		if v == f.version {
			return f.tuples
		}
		return f.snapshots[v]
	}

	return f.tuples
}

func (f *FakeRelationsRepository) formatToken() model.ConsistencyToken {
	return model.DeserializeConsistencyToken(strconv.FormatInt(f.version, 10))
}

func hasTupleInMap(tuples map[relationsTupleKey]bool, key relationsTupleKey) bool {
	return tuples[key]
}

func (f *FakeRelationsRepository) Health(_ context.Context) error {
	return nil
}

func (f *FakeRelationsRepository) Check(_ context.Context, resource model.ReporterResourceKey, relation model.Relation,
	subject model.SubjectReference, consistency model.Consistency) (bool, model.ConsistencyToken, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	token := ""
	if !consistency.MinimizeLatency() {
		token = consistency.AtLeastAsFresh().Serialize()
	}

	tuples := f.getTuplesForToken(token)
	resultToken := f.formatToken()

	subKey := subject.Subject()
	key := relationsTupleKey{
		ResourceNamespace: resource.ReporterType().Serialize(),
		ResourceType:      resource.ResourceType().Serialize(),
		ResourceID:        resource.LocalResourceId().Serialize(),
		Relation:          relation.Serialize(),
		SubjectNamespace:  subKey.ReporterType().Serialize(),
		SubjectType:       subKey.ResourceType().Serialize(),
		SubjectID:         subKey.LocalResourceId().Serialize(),
	}

	return hasTupleInMap(tuples, key), resultToken, nil
}

func (f *FakeRelationsRepository) CheckForUpdate(_ context.Context, resource model.ReporterResourceKey, relation model.Relation,
	subject model.SubjectReference) (bool, model.ConsistencyToken, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	resultToken := f.formatToken()

	subKey := subject.Subject()
	key := relationsTupleKey{
		ResourceNamespace: resource.ReporterType().Serialize(),
		ResourceType:      resource.ResourceType().Serialize(),
		ResourceID:        resource.LocalResourceId().Serialize(),
		Relation:          relation.Serialize(),
		SubjectNamespace:  subKey.ReporterType().Serialize(),
		SubjectType:       subKey.ResourceType().Serialize(),
		SubjectID:         subKey.LocalResourceId().Serialize(),
	}

	return hasTupleInMap(f.tuples, key), resultToken, nil
}

func (f *FakeRelationsRepository) CheckBulk(_ context.Context, items []model.CheckItem,
	consistency model.Consistency) ([]model.CheckBulkResultItem, model.ConsistencyToken, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	token := ""
	if !consistency.MinimizeLatency() {
		token = consistency.AtLeastAsFresh().Serialize()
	}
	tuples := f.getTuplesForToken(token)
	resultToken := f.formatToken()

	results := make([]model.CheckBulkResultItem, len(items))
	for i, item := range items {
		subKey := item.Subject.Subject()
		key := relationsTupleKey{
			ResourceNamespace: item.Resource.ReporterType().Serialize(),
			ResourceType:      item.Resource.ResourceType().Serialize(),
			ResourceID:        item.Resource.LocalResourceId().Serialize(),
			Relation:          item.Relation.Serialize(),
			SubjectNamespace:  subKey.ReporterType().Serialize(),
			SubjectType:       subKey.ResourceType().Serialize(),
			SubjectID:         subKey.LocalResourceId().Serialize(),
		}
		results[i] = model.CheckBulkResultItem{Allowed: hasTupleInMap(tuples, key)}
	}

	return results, resultToken, nil
}

func (f *FakeRelationsRepository) LookupResources(_ context.Context, query model.LookupResourcesQuery) (model.LookupResourcesIterator, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	requestedNamespace := query.ReporterType.Serialize()
	requestedType := query.ResourceType.Serialize()
	requestedRelation := query.Relation.Serialize()

	subKey := query.Subject.Subject()
	subjectNamespace := subKey.ReporterType().Serialize()
	subjectType := subKey.ResourceType().Serialize()
	subjectID := subKey.LocalResourceId().Serialize()

	var results []*model.LookupResourceResult
	for key := range f.tuples {
		namespaceMatches := requestedNamespace == "" || key.ResourceNamespace == requestedNamespace
		typeMatches := requestedType == "" || key.ResourceType == requestedType
		relationMatches := key.Relation == requestedRelation
		subjectMatches := key.SubjectNamespace == subjectNamespace &&
			key.SubjectType == subjectType &&
			key.SubjectID == subjectID

		if namespaceMatches && typeMatches && relationMatches && subjectMatches {
			resId, _ := model.NewLocalResourceId(key.ResourceID)
			resType, _ := model.NewResourceType(key.ResourceType)
			namespace, _ := model.NewReporterType(key.ResourceNamespace)
			results = append(results, &model.LookupResourceResult{
				ResourceId:   resId,
				ResourceType: resType,
				Namespace:    namespace,
			})
		}
	}

	return &fakeLookupIterator{results: results}, nil
}

func (f *FakeRelationsRepository) CreateTuples(_ context.Context, tuples []model.RelationsTuple, _ bool,
	_, _ string) (model.ConsistencyToken, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	for _, tuple := range tuples {
		key := relationsTupleKeyFromModel(tuple)
		f.tuples[key] = true
	}
	f.advanceVersion()
	return f.formatToken(), nil
}

func (f *FakeRelationsRepository) DeleteTuples(_ context.Context, tuples []model.RelationsTuple,
	_, _ string) (model.ConsistencyToken, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	for _, tuple := range tuples {
		key := relationsTupleKeyFromModel(tuple)
		delete(f.tuples, key)
	}
	f.advanceVersion()
	return f.formatToken(), nil
}

func (f *FakeRelationsRepository) AcquireLock(_ context.Context, _ string) (string, error) {
	return "fake-lock-token", nil
}

func relationsTupleKeyFromModel(tuple model.RelationsTuple) relationsTupleKey {
	return relationsTupleKey{
		ResourceNamespace: tuple.Resource().Type().Namespace(),
		ResourceType:      tuple.Resource().Type().Name(),
		ResourceID:        tuple.Resource().Id().Serialize(),
		Relation:          tuple.Relation(),
		SubjectNamespace:  tuple.Subject().Subject().Type().Namespace(),
		SubjectType:       tuple.Subject().Subject().Type().Name(),
		SubjectID:         tuple.Subject().Subject().Id().Serialize(),
	}
}

type fakeLookupIterator struct {
	results []*model.LookupResourceResult
	index   int
}

func (it *fakeLookupIterator) Next() (*model.LookupResourceResult, error) {
	if it.index >= len(it.results) {
		return nil, io.EOF
	}
	result := it.results[it.index]
	it.index++
	return result, nil
}
