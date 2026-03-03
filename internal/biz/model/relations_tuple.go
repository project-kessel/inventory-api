package model

import (
	"strings"
)

type RelationsTuple struct {
	resource RelationsResource
	relation string
	subject  RelationsSubject
}

func NewRelationsTuple(resource RelationsResource, relation string, subject RelationsSubject) RelationsTuple {
	return RelationsTuple{
		resource: resource,
		relation: relation,
		subject:  subject,
	}
}

func (rt RelationsTuple) Resource() RelationsResource {
	return rt.resource
}

func (rt RelationsTuple) Relation() string {
	return rt.relation
}

func (rt RelationsTuple) Subject() RelationsSubject {
	return rt.subject
}

// RelationsObjectType represents the type information for a resource or subject
type RelationsObjectType struct {
	name      string
	namespace string
}

func NewRelationsObjectType(name, namespace string) RelationsObjectType {
	return RelationsObjectType{
		name:      name,
		namespace: namespace,
	}
}

func (rot RelationsObjectType) Name() string {
	return strings.ToLower(rot.name)
}

func (rot RelationsObjectType) Namespace() string {
	return strings.ToLower(rot.namespace)
}

// Deprecated: Use ReporterResourceKey instead
//
// RelationsResource represents a resource in a relationship tuple
type RelationsResource struct {
	id         LocalResourceId
	objectType RelationsObjectType
}

func NewRelationsResource(id LocalResourceId, objectType RelationsObjectType) RelationsResource {
	return RelationsResource{
		id:         id,
		objectType: objectType,
	}
}

func (rr RelationsResource) Id() LocalResourceId {
	return rr.id
}

func (rr RelationsResource) Type() RelationsObjectType {
	return rr.objectType
}

// RelationsSubject represents a subject in a relationship tuple
type RelationsSubject struct {
	subject  RelationsResource // Subject is also a resource reference
	relation string
}

func NewRelationsSubject(subject RelationsResource, relation string) RelationsSubject {
	return RelationsSubject{
		subject:  subject,
		relation: relation,
	}
}

func (rs RelationsSubject) Subject() RelationsResource {
	return rs.subject
}

func (rs RelationsSubject) Relation() string {
	return strings.ToLower(rs.relation)
}

const (
	WorkspaceRelation = "workspace"
	RbacNamespace     = "rbac"
)

func NewWorkspaceRelationsTuple(workspaceID string, key ReporterResourceKey) RelationsTuple {
	resourceId := key.LocalResourceId()
	resourceType := key.ResourceType()
	reporterType := key.ReporterType()

	namespace := reporterType.String()
	resourceObjectType := NewRelationsObjectType(
		resourceType.String(),
		namespace,
	)
	resource := NewRelationsResource(resourceId, resourceObjectType)

	workspaceSubjectId, _ := NewLocalResourceId(workspaceID)
	workspaceObjectType := NewRelationsObjectType(WorkspaceRelation, RbacNamespace)
	workspaceSubject := NewRelationsResource(workspaceSubjectId, workspaceObjectType)
	/*
	 The only relation Inventory currently replicates to relations is a workspace, in which
	 the subject is the workspace itself; there should not be any subject relation.
	 Setting the subject relation to an empty string to indicate that the value should be empty in the query
	 and avoids nil which uses wildcard semantics.
	*/
	subject := NewRelationsSubject(workspaceSubject, "")

	return NewRelationsTuple(resource, WorkspaceRelation, subject)
}
