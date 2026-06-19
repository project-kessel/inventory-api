package model

// RelationsTuple represents a stored relationship fact.
// Structurally identical to Relationship but semantically distinct:
// a Relationship is a query ("does this hold?"), a RelationsTuple is a persisted fact.
type RelationsTuple struct {
	object   ResourceReference
	relation Relation
	subject  SubjectReference
}

func NewRelationsTuple(object ResourceReference, relation Relation, subject SubjectReference) RelationsTuple {
	return RelationsTuple{
		object:   object,
		relation: relation,
		subject:  subject,
	}
}

func (rt RelationsTuple) Object() ResourceReference { return rt.object }
func (rt RelationsTuple) Relation() Relation        { return rt.relation }
func (rt RelationsTuple) Subject() SubjectReference { return rt.subject }

const (
	WorkspaceRelation = "workspace"
	RbacNamespace     = "rbac"
)

func NewWorkspaceRelationsTuple(workspaceID string, key ReporterResourceKey) RelationsTuple {
	return NewRelationTupleForSubject(key, WorkspaceRelation, RbacNamespace, WorkspaceRelation, workspaceID)
}

// NewRelationTupleForSubject builds a RelationsTuple for an arbitrary relation and subject.
// This generalizes NewWorkspaceRelationsTuple to support any relation name, subject namespace,
// and subject resource type — needed for Features service relations like allowed_workspaces,
// billing_account, and parent.
func NewRelationTupleForSubject(
	key ReporterResourceKey,
	relationName string,
	subjectNamespace string,
	subjectResourceType string,
	subjectId string,
) RelationsTuple {
	reporter := NewReporterReference(key.ReporterType(), nil)
	object := NewResourceReference(
		key.ResourceType(),
		key.LocalResourceId(),
		&reporter,
	)

	subjectReporterType := DeserializeReporterType(subjectNamespace)
	subjectReporter := NewReporterReference(subjectReporterType, nil)
	subjectResource := NewResourceReference(
		DeserializeResourceType(subjectResourceType),
		DeserializeLocalResourceId(subjectId),
		&subjectReporter,
	)
	subject := NewSubjectReferenceWithoutRelation(subjectResource)

	return RelationsTuple{
		object:   object,
		relation: DeserializeRelation(relationName),
		subject:  subject,
	}
}
