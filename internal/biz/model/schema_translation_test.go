package model_test

import (
	"testing"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/project-kessel/inventory-api/internal/biz/model"
	"github.com/stretchr/testify/assert"
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

func spicedbType(ref model.ResourceReference) string {
	if !ref.HasReporter() {
		return ref.ResourceType().Serialize()
	}
	return ref.Reporter().ReporterType().Serialize() + "/" + ref.ResourceType().Serialize()
}

func TestTranslateRelationship_FeaturesWorkspaceResourceSide(t *testing.T) {
	sc := translationService()

	// features/workspace:<uuid> #enabled_services @features/service:some_service
	object := resourceRef("features", "workspace", "uuid-1")
	subject := model.NewSubjectReferenceWithoutRelation(resourceRef("features", "service", "some_service"))
	rel := model.NewRelationship(object, model.DeserializeRelation("enabled_services"), subject)

	got := sc.TranslateRelationship(rel)

	// Resource side: type folded into parent, relation prefixed.
	assert.Equal(t, "rbac/workspace", spicedbType(got.Object()))
	assert.Equal(t, "uuid-1", got.Object().ResourceId().Serialize())
	assert.Equal(t, "features_workspace_enabled_services", got.Relation().Serialize())

	// Subject side: features/service is not a derived type, left untouched.
	assert.Equal(t, "features/service", spicedbType(got.Subject().Resource()))
	assert.Equal(t, "some_service", got.Subject().Resource().ResourceId().Serialize())
}

func TestTranslateRelationship_FeaturesWorkspaceSubjectSide(t *testing.T) {
	sc := translationService()

	// A check where features/workspace is the subject: type is folded to the
	// parent, but the relation is NOT prefixed (subject side).
	object := resourceRef("hbi", "host", "host-1")
	subject := model.NewSubjectReferenceWithoutRelation(resourceRef("features", "workspace", "uuid-2"))
	rel := model.NewRelationship(object, model.DeserializeRelation("view"), subject)

	got := sc.TranslateRelationship(rel)

	assert.Equal(t, "hbi/host", spicedbType(got.Object()))
	assert.Equal(t, "view", got.Relation().Serialize(), "subject-side rewrite must not prefix the relation")
	assert.Equal(t, "rbac/workspace", spicedbType(got.Subject().Resource()))
	assert.Equal(t, "uuid-2", got.Subject().Resource().ResourceId().Serialize())
}

func TestTranslateRelationship_NonDerivedTypesUnchanged(t *testing.T) {
	sc := translationService()

	// rbac/workspace is the parent type itself, not a derived type.
	object := resourceRef("rbac", "workspace", "uuid-3")
	subject := model.NewSubjectReferenceWithoutRelation(resourceRef("rbac", "principal", "alice"))
	rel := model.NewRelationship(object, model.DeserializeRelation("inventory_host_view"), subject)

	got := sc.TranslateRelationship(rel)

	assert.Equal(t, "rbac/workspace", spicedbType(got.Object()))
	assert.Equal(t, "inventory_host_view", got.Relation().Serialize())
	assert.Equal(t, "rbac/principal", spicedbType(got.Subject().Resource()))
}

func TestTranslateRelationsTuple_FeaturesWorkspace(t *testing.T) {
	sc := translationService()

	object := resourceRef("features", "workspace", "uuid-4")
	subject := model.NewSubjectReferenceWithoutRelation(resourceRef("features", "billing_account", "acct-1"))
	tuple := model.NewRelationsTuple(object, model.DeserializeRelation("direct_billing_account"), subject)

	got := sc.TranslateRelationsTuple(tuple)

	assert.Equal(t, "rbac/workspace", spicedbType(got.Object()))
	assert.Equal(t, "features_workspace_direct_billing_account", got.Relation().Serialize())
	assert.Equal(t, "features/billing_account", spicedbType(got.Subject().Resource()))
}

func TestTranslateResourceRepresentationType_ForLookupResources(t *testing.T) {
	sc := translationService()

	reporter := model.DeserializeReporterType("features")
	rt := model.NewRepresentationType(model.DeserializeResourceType("workspace"), &reporter)

	gotType, gotRel := sc.TranslateResourceRepresentationType(rt, model.DeserializeRelation("enabled_services"))

	assert.Equal(t, "workspace", gotType.ResourceType().Serialize())
	assert.True(t, gotType.HasReporterType())
	assert.Equal(t, "rbac", gotType.ReporterType().Serialize())
	assert.Equal(t, "features_workspace_enabled_services", gotRel.Serialize())
}

func TestTranslateResourceRepresentationType_NonDerivedUnchanged(t *testing.T) {
	sc := translationService()

	reporter := model.DeserializeReporterType("rbac")
	rt := model.NewRepresentationType(model.DeserializeResourceType("workspace"), &reporter)

	gotType, gotRel := sc.TranslateResourceRepresentationType(rt, model.DeserializeRelation("inventory_host_view"))

	assert.Equal(t, "rbac", gotType.ReporterType().Serialize())
	assert.Equal(t, "workspace", gotType.ResourceType().Serialize())
	assert.Equal(t, "inventory_host_view", gotRel.Serialize())
}

func TestTranslateSubjectRepresentationType_TypeOnly(t *testing.T) {
	sc := translationService()

	reporter := model.DeserializeReporterType("features")
	rt := model.NewRepresentationType(model.DeserializeResourceType("workspace"), &reporter)

	got := sc.TranslateSubjectRepresentationType(rt)

	assert.Equal(t, "rbac", got.ReporterType().Serialize())
	assert.Equal(t, "workspace", got.ResourceType().Serialize())
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

	got := sc.TranslateTupleFilter(filter)

	// Resource side: type folded, relation prefixed.
	assert.Equal(t, "rbac", got.ReporterType().Serialize())
	assert.Equal(t, "workspace", got.ObjectType().Serialize())
	assert.Equal(t, "features_workspace_enabled_services", got.Relation().Serialize())

	// Subject side: type folded, no relation change.
	assert.Equal(t, "rbac", got.Subject().ReporterType().Serialize())
	assert.Equal(t, "workspace", got.Subject().SubjectType().Serialize())
	assert.Equal(t, "sub-uuid", got.Subject().SubjectId().Serialize())
}

func TestTranslateTupleFilter_NonDerivedUnchanged(t *testing.T) {
	sc := translationService()

	filter := model.NewTupleFilter().
		WithReporterType(model.DeserializeReporterType("rbac")).
		WithObjectType(model.DeserializeResourceType("workspace")).
		WithRelation(model.DeserializeRelation("t_parent"))

	got := sc.TranslateTupleFilter(filter)

	assert.Equal(t, "rbac", got.ReporterType().Serialize())
	assert.Equal(t, "workspace", got.ObjectType().Serialize())
	assert.Equal(t, "t_parent", got.Relation().Serialize())
}
