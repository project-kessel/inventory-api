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

func (r *recordingRelationsRepository) CheckBulk(context.Context, []model.Relationship, model.Consistency) (model.CheckBulkResult, error) {
	return model.CheckBulkResult{}, nil
}

func (r *recordingRelationsRepository) CheckForUpdateBulk(context.Context, []model.Relationship) (model.CheckBulkResult, error) {
	return model.CheckBulkResult{}, nil
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
	// the decorator must restore them to the logical type the caller queried.
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

	// Results are restored to the logical subject type the caller queried.
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

	// Results are restored to the logical type the caller queried.
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
