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

	resourceId := resource.Id()
	resourceName := strings.ToLower(resource.Type().Name())
	resourceNamespace := strings.ToLower(resource.Type().Namespace())
	relationsResource := NewRelationsResource(resourceId, NewRelationsObjectType(resourceName, resourceNamespace))

	subjectResourceId := subject.Subject().Id()
	subjectResourceName := strings.ToLower(subject.Subject().Type().Name())
	subjectResourceNamespace := strings.ToLower(subject.Subject().Type().Namespace())
	subjectResource := NewRelationsResource(subjectResourceId, NewRelationsObjectType(subjectResourceName, subjectResourceNamespace))

	return RelationsTuple{
		resource: relationsResource,
		relation: strings.ToLower(relation),
		subject:  NewRelationsSubject(subjectResource),
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
	return rot.name
}

func (rot RelationsObjectType) Namespace() string {
	return rot.namespace
}

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
	subject RelationsResource // Subject is also a resource reference
}

func NewRelationsSubject(subject RelationsResource) RelationsSubject {
	return RelationsSubject{
		subject: subject,
	}
}

func (rs RelationsSubject) Subject() RelationsResource {
	return rs.subject
}

const (
	WorkspaceRelation = "workspace"
	RbacNamespace     = "rbac"
)

func NewWorkspaceRelationsTuple(workspaceID string, key ReporterResourceKey) RelationsTuple {
	resourceId := key.LocalResourceId()
	resourceType := key.ResourceType()
	reporterType := key.ReporterType()

	namespace := strings.ToLower(reporterType.String())

	resourceObjectType := NewRelationsObjectType(
		strings.ToLower(resourceType.String()),
		namespace,
	)
	resource := NewRelationsResource(resourceId, resourceObjectType)

	workspaceSubjectId, _ := NewLocalResourceId(workspaceID)
	workspaceObjectType := NewRelationsObjectType(WorkspaceRelation, RbacNamespace)
	workspaceSubject := NewRelationsResource(workspaceSubjectId, workspaceObjectType)
	subject := NewRelationsSubject(workspaceSubject)

	return NewRelationsTuple(resource, WorkspaceRelation, subject)
}
