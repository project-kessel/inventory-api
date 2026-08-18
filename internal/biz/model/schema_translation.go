package model

// Derived-type rewrites for SpiceDB
//
// The logical schema (the unified schema language exposed by the front-facing
// API) differs from the relational schema stored in SpiceDB. In particular, a
// "subclassed"/derived resource type does not exist as its own type in SpiceDB:
// it is folded into its parent type, and its relations/permissions are prefixed
// to avoid colliding with the parent's own relations.
//
// For example, the Features workspace (reporter "features", resource type
// "workspace") is stored as the RBAC workspace ("rbac/workspace"), and a
// permission like "enabled_services" is stored as "features_workspace_enabled_services".
//
// Every request sent to SpiceDB — checks, lookups, and tuple writes/deletes —
// must therefore translate derived types before they reach the store. The rules
// are:
//   - Resource side: rewrite the type to the parent type AND prefix the relation.
//   - Subject side:  rewrite the type to the parent type only (relation unchanged).
//
// For now the rewrites are hardcoded. Once the unified/serialized schema model
// lands, these will be derived from the schema instead. See RHCLOUD-49793 / KSL-067.

const (
	featuresReporterType  = "features"
	workspaceResourceType = "workspace"
)

// subclassRewrite describes a single derived-type rewrite: how a (reporter,
// resource type) pair maps onto its parent type stored in SpiceDB.
type subclassRewrite struct {
	reporter        string
	resourceType    string
	parentNamespace string
	parentType      string
}

// relationPrefix is the prefix applied to resource-side relations of a derived
// type: reporter name + subclass type + "_" (e.g. "features_workspace_").
func (r subclassRewrite) relationPrefix() string {
	return r.reporter + "_" + r.resourceType + "_"
}

// hardcodedSubclassRewrites lists the temporary, hardcoded derived-type rewrites.
// This will be replaced by rewrites derived from the unified schema model later.
var hardcodedSubclassRewrites = []subclassRewrite{
	{
		reporter:        featuresReporterType,
		resourceType:    workspaceResourceType,
		parentNamespace: RbacNamespace,
		parentType:      workspaceResourceType,
	},
}

// lookupSubclassRewrite finds the rewrite for a (reporter, resource type) pair,
// if any. reporter must be non-empty: a reference without a reporter has no
// namespace and therefore cannot be a derived type.
func lookupSubclassRewrite(reporter, resourceType string) (subclassRewrite, bool) {
	if reporter == "" {
		return subclassRewrite{}, false
	}
	for _, r := range hardcodedSubclassRewrites {
		if r.reporter == reporter && r.resourceType == resourceType {
			return r, true
		}
	}
	return subclassRewrite{}, false
}

// TranslateRelationship rewrites both sides of a query relationship so a derived
// type is expressed against its parent type in SpiceDB. The resource side also
// gets its relation prefixed.
func (sc *SchemaService) TranslateRelationship(rel Relationship) Relationship {
	object, relation := sc.translateResourceSide(rel.Object(), rel.Relation())
	subject := sc.TranslateSubjectReference(rel.Subject())
	return NewRelationship(object, relation, subject)
}

// TranslateRelationsTuple rewrites a tuple for storage in SpiceDB, applying the
// same derived-type rewrites as queries.
func (sc *SchemaService) TranslateRelationsTuple(t RelationsTuple) RelationsTuple {
	object, relation := sc.translateResourceSide(t.Object(), t.Relation())
	subject := sc.TranslateSubjectReference(t.Subject())
	return NewRelationsTuple(object, relation, subject)
}

// TranslateResourceReference rewrites the resource side (type + relation) of a
// query, e.g. the object of a lookup-subjects request.
func (sc *SchemaService) TranslateResourceReference(ref ResourceReference, relation Relation) (ResourceReference, Relation) {
	return sc.translateResourceSide(ref, relation)
}

// TranslateSubjectReference rewrites the subject side (type only) of a query.
func (sc *SchemaService) TranslateSubjectReference(subject SubjectReference) SubjectReference {
	res := subject.Resource()
	if !res.HasReporter() {
		return subject
	}
	rw, ok := lookupSubclassRewrite(res.Reporter().ReporterType().Serialize(), res.ResourceType().Serialize())
	if !ok {
		return subject
	}
	return NewSubjectReference(rewriteReference(res, rw), subject.Relation())
}

// TranslateResourceRepresentationType rewrites the resource-side type pattern
// (type + relation) for lookup-resources requests.
func (sc *SchemaService) TranslateResourceRepresentationType(rt RepresentationType, relation Relation) (RepresentationType, Relation) {
	rw, ok := representationRewrite(rt)
	if !ok {
		return rt, relation
	}
	return rewriteRepresentationType(rw), DeserializeRelation(rw.relationPrefix() + relation.Serialize())
}

// TranslateSubjectRepresentationType rewrites the subject-side type pattern
// (type only) for lookup-subjects requests.
func (sc *SchemaService) TranslateSubjectRepresentationType(rt RepresentationType) RepresentationType {
	rw, ok := representationRewrite(rt)
	if !ok {
		return rt
	}
	return rewriteRepresentationType(rw)
}

// TranslateTupleFilter rewrites both sides of a tuple filter (used by delete and
// read). Type and reporter type are always specified together on each side.
func (sc *SchemaService) TranslateTupleFilter(f TupleFilter) TupleFilter {
	out := f

	if f.ReporterType() != nil && f.ObjectType() != nil {
		if rw, ok := lookupSubclassRewrite(f.ReporterType().Serialize(), f.ObjectType().Serialize()); ok {
			out = out.
				WithReporterType(DeserializeReporterType(rw.parentNamespace)).
				WithObjectType(DeserializeResourceType(rw.parentType))
			if f.Relation() != nil {
				out = out.WithRelation(DeserializeRelation(rw.relationPrefix() + f.Relation().Serialize()))
			}
		}
	}

	if sub := f.Subject(); sub != nil && sub.ReporterType() != nil && sub.SubjectType() != nil {
		if rw, ok := lookupSubclassRewrite(sub.ReporterType().Serialize(), sub.SubjectType().Serialize()); ok {
			newSub := sub.
				WithReporterType(DeserializeReporterType(rw.parentNamespace)).
				WithSubjectType(DeserializeResourceType(rw.parentType))
			out = out.WithSubject(newSub)
		}
	}

	return out
}

// translateResourceSide rewrites the type of a resource reference to its parent
// type and prefixes the relation. Returns the inputs unchanged when the
// reference is not a derived type.
func (sc *SchemaService) translateResourceSide(ref ResourceReference, relation Relation) (ResourceReference, Relation) {
	if !ref.HasReporter() {
		return ref, relation
	}
	rw, ok := lookupSubclassRewrite(ref.Reporter().ReporterType().Serialize(), ref.ResourceType().Serialize())
	if !ok {
		return ref, relation
	}
	return rewriteReference(ref, rw), DeserializeRelation(rw.relationPrefix() + relation.Serialize())
}

// rewriteReference builds a resource reference against the parent type. The
// reporter instance id is dropped: the parent type is owned by a different
// reporter and only its type is meaningful in SpiceDB.
func rewriteReference(ref ResourceReference, rw subclassRewrite) ResourceReference {
	reporter := NewReporterReference(DeserializeReporterType(rw.parentNamespace), nil)
	return NewResourceReference(DeserializeResourceType(rw.parentType), ref.ResourceId(), &reporter)
}

func representationRewrite(rt RepresentationType) (subclassRewrite, bool) {
	if !rt.HasReporterType() {
		return subclassRewrite{}, false
	}
	return lookupSubclassRewrite(rt.ReporterType().Serialize(), rt.ResourceType().Serialize())
}

func rewriteRepresentationType(rw subclassRewrite) RepresentationType {
	parentReporter := DeserializeReporterType(rw.parentNamespace)
	return NewRepresentationType(DeserializeResourceType(rw.parentType), &parentReporter)
}
