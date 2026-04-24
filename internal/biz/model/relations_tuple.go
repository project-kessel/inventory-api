package model

import (
	"strings"
)

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
	reporter := NewReporterReference(key.ReporterType(), nil)
	object := NewResourceReference(
		key.ResourceType(),
		key.LocalResourceId(),
		&reporter,
	)

	workspaceSubjectId := DeserializeLocalResourceId(workspaceID)
	workspaceReporterType := DeserializeReporterType(RbacNamespace)
	workspaceReporter := NewReporterReference(workspaceReporterType, nil)
	workspaceResource := NewResourceReference(
		DeserializeResourceType(WorkspaceRelation),
		workspaceSubjectId,
		&workspaceReporter,
	)
	subject := NewSubjectReferenceWithoutRelation(workspaceResource)

	return RelationsTuple{
		object:   object,
		relation: DeserializeRelation(strings.ToLower(WorkspaceRelation)),
		subject:  subject,
	}
}
