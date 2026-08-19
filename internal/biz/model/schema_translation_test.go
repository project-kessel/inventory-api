package model_test

import (
	"testing"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/project-kessel/inventory-api/internal/biz/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// translationService builds a SchemaService for translation tests. Translation
// is a pure model-to-model operation and does not touch the schema repository.
func translationService() *model.SchemaService {
	return model.NewSchemaService(nil, log.NewHelper(log.DefaultLogger))
}

func resourceRef(reporter, resourceType, id string) model.ResourceReference {
	rep := model.NewReporterReference(model.DeserializeReporterType(reporter), nil)
	return model.NewResourceReference(
		model.DeserializeResourceType(resourceType),
		model.DeserializeLocalResourceId(id),
		&rep,
	)
}

func representationType(reporter, resourceType string) model.RepresentationType {
	rep := model.DeserializeReporterType(reporter)
	return model.NewRepresentationType(model.DeserializeResourceType(resourceType), &rep)
}

func TestTranslateRelationship_FeaturesWorkspaceResourceSide(t *testing.T) {
	sc := translationService()

	// features/workspace:<uuid> #enabled_services @features/service:some_service
	object := resourceRef("features", "workspace", "uuid-1")
	subject := model.NewSubjectReferenceWithoutRelation(resourceRef("features", "service", "some_service"))
	rel := model.NewRelationship(object, model.DeserializeRelation("enabled_services"), subject)

	got := sc.TranslateRelationship(rel)

	// Resource side: type folded into parent, relation prefixed. Subject side
	// (features/service) is not a derived type, so it is left untouched.
	want := model.NewRelationship(
		resourceRef("rbac", "workspace", "uuid-1"),
		model.DeserializeRelation("features_workspace_enabled_services"),
		model.NewSubjectReferenceWithoutRelation(resourceRef("features", "service", "some_service")),
	)
	assert.Equal(t, want, got)
}

func TestTranslateRelationship_FeaturesWorkspaceSubjectSide(t *testing.T) {
	sc := translationService()

	// A check where features/workspace is the subject: type is folded to the
	// parent, but the relation is NOT prefixed (subject side).
	object := resourceRef("hbi", "host", "host-1")
	subject := model.NewSubjectReferenceWithoutRelation(resourceRef("features", "workspace", "uuid-2"))
	rel := model.NewRelationship(object, model.DeserializeRelation("view"), subject)

	got := sc.TranslateRelationship(rel)

	want := model.NewRelationship(
		resourceRef("hbi", "host", "host-1"),
		model.DeserializeRelation("view"),
		model.NewSubjectReferenceWithoutRelation(resourceRef("rbac", "workspace", "uuid-2")),
	)
	assert.Equal(t, want, got)
}

func TestTranslateRelationship_NonDerivedTypesUnchanged(t *testing.T) {
	sc := translationService()

	// rbac/workspace is the parent type itself, not a derived type.
	object := resourceRef("rbac", "workspace", "uuid-3")
	subject := model.NewSubjectReferenceWithoutRelation(resourceRef("rbac", "principal", "alice"))
	rel := model.NewRelationship(object, model.DeserializeRelation("inventory_host_view"), subject)

	got := sc.TranslateRelationship(rel)

	assert.Equal(t, rel, got)
}

func TestTranslateRelationsTuple_FeaturesWorkspace(t *testing.T) {
	sc := translationService()

	object := resourceRef("features", "workspace", "uuid-4")
	subject := model.NewSubjectReferenceWithoutRelation(resourceRef("features", "billing_account", "acct-1"))
	tuple := model.NewRelationsTuple(object, model.DeserializeRelation("direct_billing_account"), subject)

	got := sc.TranslateRelationsTuple(tuple)

	want := model.NewRelationsTuple(
		resourceRef("rbac", "workspace", "uuid-4"),
		model.DeserializeRelation("features_workspace_direct_billing_account"),
		model.NewSubjectReferenceWithoutRelation(resourceRef("features", "billing_account", "acct-1")),
	)
	assert.Equal(t, want, got)
}

// TestTranslateRelationsTuple_CommonRelationNotPrefixed covers the report-resource
// case: common/parent-owned relations (e.g. the "workspace" membership field and
// "parent" hierarchy) are folded onto the parent type but must NOT be prefixed --
// prefixing would name a relation that does not exist on rbac/workspace.
func TestTranslateRelationsTuple_CommonRelationNotPrefixed(t *testing.T) {
	sc := translationService()

	for _, commonRelation := range []string{"workspace", "parent"} {
		t.Run(commonRelation, func(t *testing.T) {
			object := resourceRef("features", "workspace", "uuid-5")
			subject := model.NewSubjectReferenceWithoutRelation(resourceRef("rbac", "workspace", "ws-1"))
			tuple := model.NewRelationsTuple(object, model.DeserializeRelation(commonRelation), subject)

			got := sc.TranslateRelationsTuple(tuple)

			want := model.NewRelationsTuple(
				resourceRef("rbac", "workspace", "uuid-5"),
				model.DeserializeRelation(commonRelation),
				model.NewSubjectReferenceWithoutRelation(resourceRef("rbac", "workspace", "ws-1")),
			)
			assert.Equal(t, want, got, "type folded to parent, common relation left unprefixed")
		})
	}
}

// TestTranslateRelationsTuple_AllOwnedRelationsPrefixed pins the full owned set so
// adding/removing an owned relation is a deliberate, tested change.
func TestTranslateRelationsTuple_AllOwnedRelationsPrefixed(t *testing.T) {
	sc := translationService()

	owned := []string{
		"direct_billing_account",
		"direct_service_preferences",
		"_paid_services",
		"_desired_services",
		"enabled_services",
	}
	for _, relation := range owned {
		t.Run(relation, func(t *testing.T) {
			object := resourceRef("features", "workspace", "uuid-6")
			subject := model.NewSubjectReferenceWithoutRelation(resourceRef("features", "service", "svc-1"))
			tuple := model.NewRelationsTuple(object, model.DeserializeRelation(relation), subject)

			got := sc.TranslateRelationsTuple(tuple)

			assert.Equal(t, "rbac/workspace", got.Object().Reporter().ReporterType().Serialize()+"/"+got.Object().ResourceType().Serialize())
			assert.Equal(t, "features_workspace_"+relation, got.Relation().Serialize())
		})
	}
}

func TestTranslateResourceRepresentationType_ForLookupResources(t *testing.T) {
	sc := translationService()

	gotType, gotRel := sc.TranslateResourceRepresentationType(representationType("features", "workspace"), model.DeserializeRelation("enabled_services"))

	assert.Equal(t, representationType("rbac", "workspace"), gotType)
	assert.Equal(t, model.DeserializeRelation("features_workspace_enabled_services"), gotRel)
}

func TestTranslateResourceRepresentationType_NonDerivedUnchanged(t *testing.T) {
	sc := translationService()

	rt := representationType("rbac", "workspace")
	rel := model.DeserializeRelation("inventory_host_view")

	gotType, gotRel := sc.TranslateResourceRepresentationType(rt, rel)

	assert.Equal(t, rt, gotType)
	assert.Equal(t, rel, gotRel)
}

func TestTranslateSubjectRepresentationType_TypeOnly(t *testing.T) {
	sc := translationService()

	got := sc.TranslateSubjectRepresentationType(representationType("features", "workspace"))

	assert.Equal(t, representationType("rbac", "workspace"), got)
}

func TestTranslateTupleFilter_BothSides(t *testing.T) {
	sc := translationService()

	subject := model.NewTupleSubjectFilter().
		WithReporterType(model.DeserializeReporterType("features")).
		WithSubjectType(model.DeserializeResourceType("workspace")).
		WithSubjectId(model.DeserializeLocalResourceId("sub-uuid"))

	filter := model.NewTupleFilter().
		WithReporterType(model.DeserializeReporterType("features")).
		WithObjectType(model.DeserializeResourceType("workspace")).
		WithRelation(model.DeserializeRelation("enabled_services")).
		WithSubject(subject)

	got, err := sc.TranslateTupleFilter(filter)
	require.NoError(t, err)

	// Resource side: type folded, relation prefixed. Subject side: type folded,
	// no relation change.
	want := model.NewTupleFilter().
		WithReporterType(model.DeserializeReporterType("rbac")).
		WithObjectType(model.DeserializeResourceType("workspace")).
		WithRelation(model.DeserializeRelation("features_workspace_enabled_services")).
		WithSubject(model.NewTupleSubjectFilter().
			WithReporterType(model.DeserializeReporterType("rbac")).
			WithSubjectType(model.DeserializeResourceType("workspace")).
			WithSubjectId(model.DeserializeLocalResourceId("sub-uuid")))
	assert.Equal(t, want, got)
}

func TestTranslateTupleFilter_NonDerivedUnchanged(t *testing.T) {
	sc := translationService()

	filter := model.NewTupleFilter().
		WithReporterType(model.DeserializeReporterType("rbac")).
		WithObjectType(model.DeserializeResourceType("workspace")).
		WithRelation(model.DeserializeRelation("t_parent"))

	got, err := sc.TranslateTupleFilter(filter)
	require.NoError(t, err)

	assert.Equal(t, filter, got)
}

func TestTranslateTupleFilter_ParentTypeWithoutRelationAllowed(t *testing.T) {
	// The guard is scoped to derived types only: deleting the parent type
	// (rbac/workspace) without a relation is unaffected and passes through as-is.
	sc := translationService()

	filter := model.NewTupleFilter().
		WithReporterType(model.DeserializeReporterType("rbac")).
		WithObjectType(model.DeserializeResourceType("workspace"))

	got, err := sc.TranslateTupleFilter(filter)
	require.NoError(t, err)

	assert.Equal(t, filter, got)
}

func TestTranslateTupleFilter_UnscopedDerivedReturnsError(t *testing.T) {
	// A derived object filter without a relation cannot be folded into the parent
	// type safely: it would match (and delete) unrelated parent-type tuples.
	sc := translationService()

	filter := model.NewTupleFilter().
		WithReporterType(model.DeserializeReporterType("features")).
		WithObjectType(model.DeserializeResourceType("workspace"))

	_, err := sc.TranslateTupleFilter(filter)
	assert.ErrorIs(t, err, model.ErrUnscopedDerivedFilter)
}
