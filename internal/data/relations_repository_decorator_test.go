package data_test

import (
	"context"
	"io"
	"testing"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/project-kessel/inventory-api/internal/biz/model"
	"github.com/project-kessel/inventory-api/internal/data"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// recordingRelationsRepository captures the arguments it receives so tests can
// assert on what the decorator forwarded to the backend. Lookup calls return a
// caller-supplied stream so result restoration can be exercised.
type recordingRelationsRepository struct {
	gotCheck        model.Relationship
	gotCreateTuples []model.RelationsTuple
	gotDeleteFilter model.TupleFilter
	deleteCalled    bool
	gotReadFilter   model.TupleFilter

	gotCheckBulkRels          []model.Relationship
	gotCheckBulkConsistency   model.Consistency
	checkBulkCalled           bool
	gotCheckForUpdateBulkRels []model.Relationship
	checkForUpdateBulkCalled  bool
	// checkBulkToken is the consistency token the fake backend returns. Result
	// pairs are synthesized by echoing the (translated) relationships it receives,
	// mirroring the real backends (see the NewCheckBulkResultPair call sites).
	checkBulkToken model.ConsistencyToken
	// omitLastBulkPair drops one pair from the response to exercise the decorator's
	// count-mismatch fallback.
	omitLastBulkPair bool

	gotLookupObjectsType     model.RepresentationType
	gotLookupObjectsRelation model.Relation
	gotLookupObjectsSubject  model.SubjectReference
	lookupObjectsStream      model.ResultStream[model.LookupObjectsItem]

	gotLookupSubjectsObject      model.ResourceReference
	gotLookupSubjectsRelation    model.Relation
	gotLookupSubjectsSubjectType model.RepresentationType
	lookupSubjectsStream         model.ResultStream[model.LookupSubjectsItem]
}

func (r *recordingRelationsRepository) Health(context.Context) (model.HealthResult, error) {
	return model.HealthResult{}, nil
}

func (r *recordingRelationsRepository) Check(_ context.Context, rel model.Relationship, _ model.Consistency) (model.CheckResult, error) {
	r.gotCheck = rel
	return model.CheckResult{}, nil
}

func (r *recordingRelationsRepository) CheckForUpdate(_ context.Context, rel model.Relationship) (model.CheckResult, error) {
	r.gotCheck = rel
	return model.CheckResult{}, nil
}

func (r *recordingRelationsRepository) CheckBulk(_ context.Context, rels []model.Relationship, consistency model.Consistency) (model.CheckBulkResult, error) {
	r.checkBulkCalled = true
	r.gotCheckBulkRels = rels
	r.gotCheckBulkConsistency = consistency
	return r.echoBulkResult(rels), nil
}

func (r *recordingRelationsRepository) CheckForUpdateBulk(_ context.Context, rels []model.Relationship) (model.CheckBulkResult, error) {
	r.checkForUpdateBulkCalled = true
	r.gotCheckForUpdateBulkRels = rels
	return r.echoBulkResult(rels), nil
}

// echoBulkResult mimics the real backends, which build each result pair from the
// request they received. The decorator hands them translated relationships, so
// without restoration these pairs would carry the serialized form back to the caller.
func (r *recordingRelationsRepository) echoBulkResult(rels []model.Relationship) model.CheckBulkResult {
	pairs := make([]model.CheckBulkResultPair, len(rels))
	for i, rel := range rels {
		pairs[i] = model.NewCheckBulkResultPair(rel, model.NewCheckBulkResultItem(true, nil, 0))
	}
	if r.omitLastBulkPair && len(pairs) > 0 {
		pairs = pairs[:len(pairs)-1]
	}
	return model.NewCheckBulkResult(pairs, r.checkBulkToken)
}

func (r *recordingRelationsRepository) LookupObjects(_ context.Context, objectType model.RepresentationType, relation model.Relation, subject model.SubjectReference, _ *model.Pagination, _ model.Consistency) (model.ResultStream[model.LookupObjectsItem], error) {
	r.gotLookupObjectsType = objectType
	r.gotLookupObjectsRelation = relation
	r.gotLookupObjectsSubject = subject
	return r.lookupObjectsStream, nil
}

func (r *recordingRelationsRepository) LookupSubjects(_ context.Context, object model.ResourceReference, relation model.Relation, subjectType model.RepresentationType, _ *model.Relation, _ *model.Pagination, _ model.Consistency) (model.ResultStream[model.LookupSubjectsItem], error) {
	r.gotLookupSubjectsObject = object
	r.gotLookupSubjectsRelation = relation
	r.gotLookupSubjectsSubjectType = subjectType
	return r.lookupSubjectsStream, nil
}

func (r *recordingRelationsRepository) CreateTuples(_ context.Context, tuples []model.RelationsTuple, _ bool, _ *model.FencingCheck) (model.TuplesResult, error) {
	r.gotCreateTuples = tuples
	return model.TuplesResult{}, nil
}

func (r *recordingRelationsRepository) DeleteTuples(_ context.Context, filter model.TupleFilter, _ *model.FencingCheck) (model.TuplesResult, error) {
	r.deleteCalled = true
	r.gotDeleteFilter = filter
	return model.TuplesResult{}, nil
}

func (r *recordingRelationsRepository) ReadTuples(_ context.Context, filter model.TupleFilter, _ *model.Pagination, _ model.Consistency) (model.ResultStream[model.ReadTuplesItem], error) {
	r.gotReadFilter = filter
	return nil, nil
}

func (r *recordingRelationsRepository) AcquireLock(context.Context, model.LockId) (model.AcquireLockResult, error) {
	return model.AcquireLockResult{}, nil
}

// sliceStream yields a fixed set of items then io.EOF.
type sliceStream[T any] struct {
	items []T
	pos   int
}

func (s *sliceStream[T]) Recv() (T, error) {
	var zero T
	if s.pos >= len(s.items) {
		return zero, io.EOF
	}
	item := s.items[s.pos]
	s.pos++
	return item, nil
}

func newDecorator(inner model.RelationsRepository) *data.RelationsRepositoryDecorator {
	schema := model.NewSchemaService(nil, log.NewHelper(log.DefaultLogger))
	return data.NewRelationsRepositoryDecorator(inner, schema)
}

func resourceRef(reporter, resourceType, id string) model.ResourceReference {
	rep := model.NewReporterReference(model.DeserializeReporterType(reporter), nil)
	return model.NewResourceReference(
		model.DeserializeResourceType(resourceType),
		model.DeserializeLocalResourceId(id),
		&rep,
	)
}

func spicedbType(ref model.ResourceReference) string {
	if !ref.HasReporter() {
		return ref.ResourceType().Serialize()
	}
	return ref.Reporter().ReporterType().Serialize() + "/" + ref.ResourceType().Serialize()
}

// derivedRel builds a relationship on the derived features/workspace type, whose
// object side must be folded to rbac/workspace and its relation prefixed.
func derivedRel(id string) model.Relationship {
	return model.NewRelationship(
		resourceRef("features", "workspace", id),
		model.DeserializeRelation("enabled_services"),
		model.NewSubjectReferenceWithoutRelation(resourceRef("features", "service", "svc-"+id)),
	)
}

// nonDerivedRel builds a relationship on a plain (non-derived) type that must be
// forwarded to the backend untouched.
func nonDerivedRel(id string) model.Relationship {
	return model.NewRelationship(
		resourceRef("rbac", "role_binding", id),
		model.DeserializeRelation("subject"),
		model.NewSubjectReferenceWithoutRelation(resourceRef("rbac", "principal", "alice")),
	)
}

func TestDecorator_CheckTranslatesResourceSide(t *testing.T) {
	inner := &recordingRelationsRepository{}
	repo := newDecorator(inner)

	rel := model.NewRelationship(
		resourceRef("features", "workspace", "uuid-1"),
		model.DeserializeRelation("enabled_services"),
		model.NewSubjectReferenceWithoutRelation(resourceRef("features", "service", "svc-1")),
	)

	_, err := repo.Check(context.Background(), rel, model.NewConsistencyMinimizeLatency())
	require.NoError(t, err)

	assert.Equal(t, "rbac/workspace", spicedbType(inner.gotCheck.Object()))
	assert.Equal(t, "features_workspace_enabled_services", inner.gotCheck.Relation().Serialize())
	assert.Equal(t, "features/service", spicedbType(inner.gotCheck.Subject().Resource()))
}

func TestDecorator_CheckBulkTranslatesQueryAndRestoresResult(t *testing.T) {
	token, err := model.NewConsistencyToken("tok-123")
	require.NoError(t, err)
	inner := &recordingRelationsRepository{checkBulkToken: token}
	repo := newDecorator(inner)

	rels := []model.Relationship{derivedRel("uuid-1"), derivedRel("uuid-2")}

	res, err := repo.CheckBulk(context.Background(), rels, model.NewConsistencyAtLeastAsFresh(token))
	require.NoError(t, err)

	// (a)/(c-in) Query side: each relationship folded to the parent type and its
	// relation prefixed, with input order preserved.
	require.Len(t, inner.gotCheckBulkRels, 2)
	for i, got := range inner.gotCheckBulkRels {
		assert.Equal(t, "rbac/workspace", spicedbType(got.Object()), "rel %d object", i)
		assert.Equal(t, "features_workspace_enabled_services", got.Relation().Serialize(), "rel %d relation", i)
	}

	// Consistency is forwarded unchanged.
	assert.Equal(t, model.ConsistencyAtLeastAsFresh, model.ConsistencyTypeOf(inner.gotCheckBulkConsistency))
	gotToken := model.ConsistencyAtLeastAsFreshToken(inner.gotCheckBulkConsistency)
	require.NotNil(t, gotToken)
	assert.Equal(t, token, *gotToken)

	// (b)/(c-return) Result side: pairs restored to the kessel type and
	// un-prefixed relation the caller supplied -- the serialized form never leaks back.
	require.Len(t, res.Pairs(), 2)
	for i, p := range res.Pairs() {
		assert.Equal(t, "features/workspace", spicedbType(p.Request().Object()), "pair %d object", i)
		assert.Equal(t, "enabled_services", p.Request().Relation().Serialize(), "pair %d relation", i)
		assert.True(t, p.Result().Allowed(), "pair %d result", i)
	}
	assert.Equal(t, "uuid-1", res.Pairs()[0].Request().Object().ResourceId().Serialize())
	assert.Equal(t, "uuid-2", res.Pairs()[1].Request().Object().ResourceId().Serialize())
	// Consistency token preserved.
	assert.Equal(t, token, res.ConsistencyToken())
}

func TestDecorator_CheckBulkRestoresOnlyDerivedInMixedInput(t *testing.T) {
	inner := &recordingRelationsRepository{}
	repo := newDecorator(inner)

	rels := []model.Relationship{derivedRel("uuid-1"), nonDerivedRel("uuid-2")}

	res, err := repo.CheckBulk(context.Background(), rels, model.NewConsistencyMinimizeLatency())
	require.NoError(t, err)

	// Query side: derived folded + prefixed, non-derived untouched.
	require.Len(t, inner.gotCheckBulkRels, 2)
	assert.Equal(t, "rbac/workspace", spicedbType(inner.gotCheckBulkRels[0].Object()))
	assert.Equal(t, "features_workspace_enabled_services", inner.gotCheckBulkRels[0].Relation().Serialize())
	assert.Equal(t, "rbac/role_binding", spicedbType(inner.gotCheckBulkRels[1].Object()))
	assert.Equal(t, "subject", inner.gotCheckBulkRels[1].Relation().Serialize())

	// Result side: derived restored to kessel, non-derived unchanged.
	require.Len(t, res.Pairs(), 2)
	assert.Equal(t, "features/workspace", spicedbType(res.Pairs()[0].Request().Object()))
	assert.Equal(t, "enabled_services", res.Pairs()[0].Request().Relation().Serialize())
	assert.Equal(t, "rbac/role_binding", spicedbType(res.Pairs()[1].Request().Object()))
	assert.Equal(t, "subject", res.Pairs()[1].Request().Relation().Serialize())
}

func TestDecorator_CheckBulkForwardsEmptyInput(t *testing.T) {
	cases := map[string][]model.Relationship{
		"empty": {},
		"nil":   nil,
	}
	for name, input := range cases {
		t.Run(name, func(t *testing.T) {
			inner := &recordingRelationsRepository{}
			repo := newDecorator(inner)

			res, err := repo.CheckBulk(context.Background(), input, model.NewConsistencyMinimizeLatency())
			require.NoError(t, err)

			assert.True(t, inner.checkBulkCalled, "backend CheckBulk must still be called")
			assert.NotNil(t, inner.gotCheckBulkRels, "decorator should forward a non-nil slice")
			assert.Empty(t, inner.gotCheckBulkRels)
			assert.Empty(t, res.Pairs())
		})
	}
}

func TestDecorator_CheckBulkLeavesResultUnchangedOnCountMismatch(t *testing.T) {
	// Defensive: if the backend returns a different number of pairs than were
	// requested, the decorator cannot positionally restore, so it forwards the
	// result unchanged and lets the usecase's length check surface the error.
	inner := &recordingRelationsRepository{omitLastBulkPair: true}
	repo := newDecorator(inner)

	rels := []model.Relationship{derivedRel("uuid-1"), derivedRel("uuid-2")}

	res, err := repo.CheckBulk(context.Background(), rels, model.NewConsistencyMinimizeLatency())
	require.NoError(t, err)

	// One pair returned for two requests: passed through untouched, still serialized.
	require.Len(t, res.Pairs(), 1)
	assert.Equal(t, "rbac/workspace", spicedbType(res.Pairs()[0].Request().Object()))
	assert.Equal(t, "features_workspace_enabled_services", res.Pairs()[0].Request().Relation().Serialize())
}

func TestDecorator_CheckForUpdateBulkTranslatesQueryAndRestoresResult(t *testing.T) {
	token, err := model.NewConsistencyToken("tok-9")
	require.NoError(t, err)
	inner := &recordingRelationsRepository{checkBulkToken: token}
	repo := newDecorator(inner)

	rels := []model.Relationship{derivedRel("uuid-1"), nonDerivedRel("uuid-2")}

	res, err := repo.CheckForUpdateBulk(context.Background(), rels)
	require.NoError(t, err)

	// Query side: derived translated, non-derived untouched.
	require.Len(t, inner.gotCheckForUpdateBulkRels, 2)
	assert.Equal(t, "rbac/workspace", spicedbType(inner.gotCheckForUpdateBulkRels[0].Object()))
	assert.Equal(t, "features_workspace_enabled_services", inner.gotCheckForUpdateBulkRels[0].Relation().Serialize())
	assert.Equal(t, "rbac/role_binding", spicedbType(inner.gotCheckForUpdateBulkRels[1].Object()))
	assert.Equal(t, "subject", inner.gotCheckForUpdateBulkRels[1].Relation().Serialize())

	// Result side: pairs restored to the kessel form the caller supplied.
	require.Len(t, res.Pairs(), 2)
	assert.Equal(t, "features/workspace", spicedbType(res.Pairs()[0].Request().Object()))
	assert.Equal(t, "enabled_services", res.Pairs()[0].Request().Relation().Serialize())
	assert.Equal(t, "rbac/role_binding", spicedbType(res.Pairs()[1].Request().Object()))
	assert.Equal(t, "subject", res.Pairs()[1].Request().Relation().Serialize())
	assert.Equal(t, token, res.ConsistencyToken())
}

func TestDecorator_CreateTuplesTranslatesEach(t *testing.T) {
	inner := &recordingRelationsRepository{}
	repo := newDecorator(inner)

	tuples := []model.RelationsTuple{
		model.NewRelationsTuple(
			resourceRef("features", "workspace", "uuid-2"),
			model.DeserializeRelation("direct_billing_account"),
			model.NewSubjectReferenceWithoutRelation(resourceRef("features", "billing_account", "acct-1")),
		),
	}

	_, err := repo.CreateTuples(context.Background(), tuples, true, nil)
	require.NoError(t, err)

	require.Len(t, inner.gotCreateTuples, 1)
	assert.Equal(t, "rbac/workspace", spicedbType(inner.gotCreateTuples[0].Object()))
	assert.Equal(t, "features_workspace_direct_billing_account", inner.gotCreateTuples[0].Relation().Serialize())
}

func TestDecorator_CreateTuplesLeavesCommonRelationUnprefixed(t *testing.T) {
	// report-resource emits common/parent-owned relations (e.g. the "workspace"
	// membership field) on a derived type. The type is folded to the parent but
	// the relation must NOT be prefixed, or the tuple lands under a relation that
	// does not exist on rbac/workspace.
	inner := &recordingRelationsRepository{}
	repo := newDecorator(inner)

	tuples := []model.RelationsTuple{
		model.NewRelationsTuple(
			resourceRef("features", "workspace", "uuid-3"),
			model.DeserializeRelation("workspace"),
			model.NewSubjectReferenceWithoutRelation(resourceRef("rbac", "workspace", "ws-1")),
		),
	}

	_, err := repo.CreateTuples(context.Background(), tuples, true, nil)
	require.NoError(t, err)

	require.Len(t, inner.gotCreateTuples, 1)
	assert.Equal(t, "rbac/workspace", spicedbType(inner.gotCreateTuples[0].Object()))
	assert.Equal(t, "workspace", inner.gotCreateTuples[0].Relation().Serialize())
}

func TestDecorator_DeleteTuplesLeavesCommonRelationUnprefixed(t *testing.T) {
	// Symmetric with create: a delete filtering on a common relation folds the
	// type to the parent and forwards the relation unprefixed (not rejected).
	inner := &recordingRelationsRepository{}
	repo := newDecorator(inner)

	filter := model.NewTupleFilter().
		WithReporterType(model.DeserializeReporterType("features")).
		WithObjectType(model.DeserializeResourceType("workspace")).
		WithRelation(model.DeserializeRelation("parent"))

	_, err := repo.DeleteTuples(context.Background(), filter, nil)
	require.NoError(t, err)

	assert.True(t, inner.deleteCalled)
	assert.Equal(t, "rbac", inner.gotDeleteFilter.ReporterType().Serialize())
	assert.Equal(t, "workspace", inner.gotDeleteFilter.ObjectType().Serialize())
	require.NotNil(t, inner.gotDeleteFilter.Relation())
	assert.Equal(t, "parent", inner.gotDeleteFilter.Relation().Serialize())
}

func TestDecorator_DeleteTuplesTranslatesFilter(t *testing.T) {
	inner := &recordingRelationsRepository{}
	repo := newDecorator(inner)

	filter := model.NewTupleFilter().
		WithReporterType(model.DeserializeReporterType("features")).
		WithObjectType(model.DeserializeResourceType("workspace")).
		WithRelation(model.DeserializeRelation("enabled_services"))

	_, err := repo.DeleteTuples(context.Background(), filter, nil)
	require.NoError(t, err)

	assert.Equal(t, "rbac", inner.gotDeleteFilter.ReporterType().Serialize())
	assert.Equal(t, "workspace", inner.gotDeleteFilter.ObjectType().Serialize())
	assert.Equal(t, "features_workspace_enabled_services", inner.gotDeleteFilter.Relation().Serialize())
}

func TestDecorator_DeleteTuplesRejectsUnscopedDerivedFilter(t *testing.T) {
	// A derived-type delete without a relation would fold to the parent type
	// (rbac/workspace) unscoped and wipe unrelated parent tuples. It must error
	// and never reach the backend.
	inner := &recordingRelationsRepository{}
	repo := newDecorator(inner)

	filter := model.NewTupleFilter().
		WithReporterType(model.DeserializeReporterType("features")).
		WithObjectType(model.DeserializeResourceType("workspace"))

	_, err := repo.DeleteTuples(context.Background(), filter, nil)
	require.ErrorIs(t, err, model.ErrUnscopedDerivedFilter)
	assert.False(t, inner.deleteCalled, "backend DeleteTuples must not be called for an unsafe filter")
}

func TestDecorator_ReadTuplesForwardsFilterUntranslated(t *testing.T) {
	// ReadTuples is the deprecated RBAC-only raw SpiceDB bypass: the filter must
	// reach the backend unchanged (no type folding, no relation prefix).
	inner := &recordingRelationsRepository{}
	repo := newDecorator(inner)

	filter := model.NewTupleFilter().
		WithReporterType(model.DeserializeReporterType("features")).
		WithObjectType(model.DeserializeResourceType("workspace")).
		WithRelation(model.DeserializeRelation("enabled_services"))

	_, err := repo.ReadTuples(context.Background(), filter, nil, model.NewConsistencyMinimizeLatency())
	require.NoError(t, err)

	assert.Equal(t, "features", inner.gotReadFilter.ReporterType().Serialize())
	assert.Equal(t, "workspace", inner.gotReadFilter.ObjectType().Serialize())
	assert.Equal(t, "enabled_services", inner.gotReadFilter.Relation().Serialize())
}

func TestDecorator_LookupObjectsNonDerivedForwardsUnchanged(t *testing.T) {
	// A non-derived type is neither translated on the way in nor restored on the
	// way out: the raw backend stream is returned as-is.
	backendStream := &sliceStream[model.LookupObjectsItem]{items: []model.LookupObjectsItem{
		model.NewLookupObjectsItem(resourceRef("rbac", "workspace", "uuid-a"), model.ContinuationToken("")),
	}}
	inner := &recordingRelationsRepository{lookupObjectsStream: backendStream}
	repo := newDecorator(inner)

	reporter := model.DeserializeReporterType("rbac")
	objectType := model.NewRepresentationType(model.DeserializeResourceType("workspace"), &reporter)

	stream, err := repo.LookupObjects(
		context.Background(),
		objectType,
		model.DeserializeRelation("inventory_host_view"),
		model.NewSubjectReferenceWithoutRelation(resourceRef("rbac", "principal", "alice")),
		nil,
		model.NewConsistencyMinimizeLatency(),
	)
	require.NoError(t, err)

	// Request forwarded unchanged.
	assert.Equal(t, "rbac", inner.gotLookupObjectsType.ReporterType().Serialize())
	assert.Equal(t, "workspace", inner.gotLookupObjectsType.ResourceType().Serialize())
	assert.Equal(t, "inventory_host_view", inner.gotLookupObjectsRelation.Serialize())
	// The exact backend stream is returned (not wrapped in a restoring stream).
	assert.Same(t, backendStream, stream)
}

func TestDecorator_LookupSubjectsTranslatesRequestAndRestoresResults(t *testing.T) {
	// The backend returns subjects labeled with the translated (parent) type;
	// the decorator must restore them to the kessel type the caller queried.
	inner := &recordingRelationsRepository{
		lookupSubjectsStream: &sliceStream[model.LookupSubjectsItem]{items: []model.LookupSubjectsItem{
			model.NewLookupSubjectsItem(model.NewSubjectReferenceWithoutRelation(resourceRef("rbac", "workspace", "uuid-a")), model.ContinuationToken("")),
			model.NewLookupSubjectsItem(model.NewSubjectReferenceWithoutRelation(resourceRef("rbac", "workspace", "uuid-b")), model.ContinuationToken("")),
		}},
	}
	repo := newDecorator(inner)

	reporter := model.DeserializeReporterType("features")
	subjectType := model.NewRepresentationType(model.DeserializeResourceType("workspace"), &reporter)

	stream, err := repo.LookupSubjects(
		context.Background(),
		resourceRef("features", "workspace", "obj-1"),
		model.DeserializeRelation("enabled_services"),
		subjectType,
		nil,
		nil,
		model.NewConsistencyMinimizeLatency(),
	)
	require.NoError(t, err)

	// Object side (resource side): folded to parent AND relation prefixed.
	assert.Equal(t, "rbac/workspace", spicedbType(inner.gotLookupSubjectsObject))
	assert.Equal(t, "features_workspace_enabled_services", inner.gotLookupSubjectsRelation.Serialize())
	// Subject-type side: folded to parent, relation NOT prefixed.
	assert.Equal(t, "workspace", inner.gotLookupSubjectsSubjectType.ResourceType().Serialize())
	assert.Equal(t, "rbac", inner.gotLookupSubjectsSubjectType.ReporterType().Serialize())

	// Results are restored to the kessel subject type the caller queried.
	first, err := stream.Recv()
	require.NoError(t, err)
	assert.Equal(t, "features/workspace", spicedbType(first.Subject().Resource()))
	assert.Equal(t, "uuid-a", first.Subject().Resource().ResourceId().Serialize())

	second, err := stream.Recv()
	require.NoError(t, err)
	assert.Equal(t, "features/workspace", spicedbType(second.Subject().Resource()))
	assert.Equal(t, "uuid-b", second.Subject().Resource().ResourceId().Serialize())

	_, err = stream.Recv()
	assert.ErrorIs(t, err, io.EOF)
}

func TestDecorator_LookupObjectsTranslatesRequestAndRestoresResults(t *testing.T) {
	// The backend returns objects labeled with the translated (parent) type,
	// mirroring how the real SpiceDB stream stamps the request type onto results.
	inner := &recordingRelationsRepository{
		lookupObjectsStream: &sliceStream[model.LookupObjectsItem]{items: []model.LookupObjectsItem{
			model.NewLookupObjectsItem(resourceRef("rbac", "workspace", "uuid-a"), model.ContinuationToken("")),
			model.NewLookupObjectsItem(resourceRef("rbac", "workspace", "uuid-b"), model.ContinuationToken("")),
		}},
	}
	repo := newDecorator(inner)

	reporter := model.DeserializeReporterType("features")
	objectType := model.NewRepresentationType(model.DeserializeResourceType("workspace"), &reporter)

	stream, err := repo.LookupObjects(
		context.Background(),
		objectType,
		model.DeserializeRelation("enabled_services"),
		model.NewSubjectReferenceWithoutRelation(resourceRef("rbac", "principal", "alice")),
		nil,
		model.NewConsistencyMinimizeLatency(),
	)
	require.NoError(t, err)

	// Request was translated to the parent type + prefixed relation.
	assert.Equal(t, "workspace", inner.gotLookupObjectsType.ResourceType().Serialize())
	assert.Equal(t, "rbac", inner.gotLookupObjectsType.ReporterType().Serialize())
	assert.Equal(t, "features_workspace_enabled_services", inner.gotLookupObjectsRelation.Serialize())

	// Results are restored to the kessel type the caller queried.
	first, err := stream.Recv()
	require.NoError(t, err)
	assert.Equal(t, "features/workspace", spicedbType(first.Object()))
	assert.Equal(t, "uuid-a", first.Object().ResourceId().Serialize())

	second, err := stream.Recv()
	require.NoError(t, err)
	assert.Equal(t, "features/workspace", spicedbType(second.Object()))
	assert.Equal(t, "uuid-b", second.Object().ResourceId().Serialize())

	_, err = stream.Recv()
	assert.ErrorIs(t, err, io.EOF)
}
