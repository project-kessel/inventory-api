package data

import (
	"context"

	"github.com/project-kessel/inventory-api/internal/biz/model"
)

// RelationsRepositoryDecorator decorates a RelationsRepository, rewriting
// derived/subclassed resource types from the kessel schema into the serialized
// schema before every request reaches the backend. Because the kessel schema
// exposed by the API differs from the serialized schema stored in the backend,
// this decorator is the single chokepoint where the kessel schema is translated
// into the serialized schema for backend calls (checks, lookups, and tuple
// writes/deletes).
//
// Results are restored to the kessel type the caller queried: a lookup against
// features/workspace is sent to the backend as rbac/workspace, but the returned
// objects are relabeled features/workspace so the API stays consistent with the
// kessel schema. Bulk checks are restored the same way -- each result pair
// echoes back the original kessel relationship, not the serialized one sent to
// the backend.
//
// ReadTuples is deliberately exempt from translation in both directions -- see
// the note on that method.
//
// The translation itself is owned by the SchemaService; this type only decides
// where to apply it. See RHCLOUD-49793 / KSL-067.
type RelationsRepositoryDecorator struct {
	inner  model.RelationsRepository
	schema *model.SchemaService
}

// NewRelationsRepositoryDecorator wraps inner so that all requests are
// translated by schema before being forwarded.
func NewRelationsRepositoryDecorator(inner model.RelationsRepository, schema *model.SchemaService) *RelationsRepositoryDecorator {
	return &RelationsRepositoryDecorator{inner: inner, schema: schema}
}

func (t *RelationsRepositoryDecorator) Health(ctx context.Context) (model.HealthResult, error) {
	return t.inner.Health(ctx)
}

func (t *RelationsRepositoryDecorator) Check(ctx context.Context, rel model.Relationship, consistency model.Consistency) (model.CheckResult, error) {
	return t.inner.Check(ctx, t.schema.TranslateRelationship(rel), consistency)
}

func (t *RelationsRepositoryDecorator) CheckForUpdate(ctx context.Context, rel model.Relationship) (model.CheckResult, error) {
	return t.inner.CheckForUpdate(ctx, t.schema.TranslateRelationship(rel))
}

func (t *RelationsRepositoryDecorator) CheckBulk(ctx context.Context, rels []model.Relationship, consistency model.Consistency) (model.CheckBulkResult, error) {
	result, err := t.inner.CheckBulk(ctx, t.translateRelationships(rels), consistency)
	if err != nil {
		return result, err
	}
	return restoreBulkResult(rels, result), nil
}

func (t *RelationsRepositoryDecorator) CheckForUpdateBulk(ctx context.Context, rels []model.Relationship) (model.CheckBulkResult, error) {
	result, err := t.inner.CheckForUpdateBulk(ctx, t.translateRelationships(rels))
	if err != nil {
		return result, err
	}
	return restoreBulkResult(rels, result), nil
}

func (t *RelationsRepositoryDecorator) LookupObjects(
	ctx context.Context,
	objectType model.RepresentationType,
	relation model.Relation,
	subject model.SubjectReference,
	pagination *model.Pagination,
	consistency model.Consistency,
) (model.ResultStream[model.LookupObjectsItem], error) {
	translatedType, translatedRelation := t.schema.TranslateResourceRepresentationType(objectType, relation)
	translatedSubject := t.schema.TranslateSubjectReference(subject)

	stream, err := t.inner.LookupObjects(ctx, translatedType, translatedRelation, translatedSubject, pagination, consistency)
	if err != nil {
		return nil, err
	}

	if !representationTypeChanged(objectType, translatedType) {
		return stream, nil
	}
	// Restore the kessel type the caller queried onto each result.
	return &restoringLookupObjectsStream{inner: stream, kesselType: objectType}, nil
}

func (t *RelationsRepositoryDecorator) LookupSubjects(
	ctx context.Context,
	object model.ResourceReference,
	relation model.Relation,
	subjectType model.RepresentationType,
	subjectRelation *model.Relation,
	pagination *model.Pagination,
	consistency model.Consistency,
) (model.ResultStream[model.LookupSubjectsItem], error) {
	translatedObject, translatedRelation := t.schema.TranslateResourceReference(object, relation)
	translatedSubjectType := t.schema.TranslateSubjectRepresentationType(subjectType)

	stream, err := t.inner.LookupSubjects(ctx, translatedObject, translatedRelation, translatedSubjectType, subjectRelation, pagination, consistency)
	if err != nil {
		return nil, err
	}

	if !representationTypeChanged(subjectType, translatedSubjectType) {
		return stream, nil
	}
	return &restoringLookupSubjectsStream{inner: stream, kesselType: subjectType}, nil
}

func (t *RelationsRepositoryDecorator) CreateTuples(ctx context.Context, tuples []model.RelationsTuple, upsert bool, fencing *model.FencingCheck) (model.TuplesResult, error) {
	translated := make([]model.RelationsTuple, len(tuples))
	for i, tuple := range tuples {
		translated[i] = t.schema.TranslateRelationsTuple(tuple)
	}
	return t.inner.CreateTuples(ctx, translated, upsert, fencing)
}

func (t *RelationsRepositoryDecorator) DeleteTuples(ctx context.Context, filter model.TupleFilter, fencing *model.FencingCheck) (model.TuplesResult, error) {
	translated, err := t.schema.TranslateTupleFilter(filter)
	if err != nil {
		return model.TuplesResult{}, err
	}
	return t.inner.DeleteTuples(ctx, translated, fencing)
}

// ReadTuples is the deprecated, RBAC-only raw SpiceDB bypass (see
// KesselTupleService in api/kessel/inventory/v1beta2/tuple_service.proto and
// TupleCrudUseCase, both marked deprecated and slated for removal). It is not
// part of the kessel API, so it is intentionally left untranslated in both
// directions: the filter is forwarded as-is and results are returned as-is,
// exposing the serialized schema directly. Callers of this bypass are
// expected to know they operate against the serialized schema.
func (t *RelationsRepositoryDecorator) ReadTuples(ctx context.Context, filter model.TupleFilter, pagination *model.Pagination, consistency model.Consistency) (model.ResultStream[model.ReadTuplesItem], error) {
	return t.inner.ReadTuples(ctx, filter, pagination, consistency)
}

func (t *RelationsRepositoryDecorator) AcquireLock(ctx context.Context, lockId model.LockId) (model.AcquireLockResult, error) {
	return t.inner.AcquireLock(ctx, lockId)
}

func (t *RelationsRepositoryDecorator) translateRelationships(rels []model.Relationship) []model.Relationship {
	translated := make([]model.Relationship, len(rels))
	for i, rel := range rels {
		translated[i] = t.schema.TranslateRelationship(rel)
	}
	return translated
}

// restoreBulkResult relabels each result pair with the original kessel
// relationship the caller supplied. The backend builds its pairs from the
// translated relationships it received, so without this step the serialized form
// (e.g. rbac/workspace and the prefixed relation) would leak back through the
// kessel API. Pairs are matched to inputs positionally -- backends preserve
// request order. If the counts disagree the result is returned unchanged so the
// mismatch is surfaced upstream rather than mispaired here.
func restoreBulkResult(original []model.Relationship, result model.CheckBulkResult) model.CheckBulkResult {
	pairs := result.Pairs()
	if len(pairs) != len(original) {
		return result
	}
	restored := make([]model.CheckBulkResultPair, len(pairs))
	for i, pair := range pairs {
		restored[i] = model.NewCheckBulkResultPair(original[i], pair.Result())
	}
	return model.NewCheckBulkResult(restored, result.ConsistencyToken())
}

// representationTypeChanged reports whether translation altered the type pattern,
// which is the signal that lookup results need to be restored to the kessel type.
func representationTypeChanged(before, after model.RepresentationType) bool {
	if before.ResourceType().Serialize() != after.ResourceType().Serialize() {
		return true
	}
	return reporterTypeString(before) != reporterTypeString(after)
}

func reporterTypeString(rt model.RepresentationType) string {
	if !rt.HasReporterType() {
		return ""
	}
	return rt.ReporterType().Serialize()
}

// restoringLookupObjectsStream relabels each looked-up object with the kessel
// type the caller requested, keeping the id returned by the backend.
type restoringLookupObjectsStream struct {
	inner      model.ResultStream[model.LookupObjectsItem]
	kesselType model.RepresentationType
}

func (s *restoringLookupObjectsStream) Recv() (model.LookupObjectsItem, error) {
	item, err := s.inner.Recv()
	if err != nil {
		return model.LookupObjectsItem{}, err
	}
	ref := model.NewResourceReference(
		s.kesselType.ResourceType(),
		item.Object().ResourceId(),
		reporterReferenceFor(s.kesselType),
	)
	return model.NewLookupObjectsItem(ref, item.ContinuationToken()), nil
}

// restoringLookupSubjectsStream relabels each looked-up subject with the kessel
// subject type the caller requested, keeping the id returned by the backend.
type restoringLookupSubjectsStream struct {
	inner      model.ResultStream[model.LookupSubjectsItem]
	kesselType model.RepresentationType
}

func (s *restoringLookupSubjectsStream) Recv() (model.LookupSubjectsItem, error) {
	item, err := s.inner.Recv()
	if err != nil {
		return model.LookupSubjectsItem{}, err
	}
	ref := model.NewResourceReference(
		s.kesselType.ResourceType(),
		item.Subject().Resource().ResourceId(),
		reporterReferenceFor(s.kesselType),
	)
	subject := model.NewSubjectReference(ref, item.Subject().Relation())
	return model.NewLookupSubjectsItem(subject, item.ContinuationToken()), nil
}

func reporterReferenceFor(rt model.RepresentationType) *model.ReporterReference {
	if !rt.HasReporterType() {
		return nil
	}
	ref := model.NewReporterReference(*rt.ReporterType(), nil)
	return &ref
}
