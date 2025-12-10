package model

import (
	"errors"
)

// ObjectType represents a type of object in the authorization system
type ObjectType struct {
	Namespace string
	Name      string
}

func NewObjectType(namespace, name string) (*ObjectType, error) {
	if namespace == "" {
		return nil, errors.New("namespace cannot be empty")
	}
	if name == "" {
		return nil, errors.New("name cannot be empty")
	}
	return &ObjectType{Namespace: namespace, Name: name}, nil
}

// ObjectReference uniquely identifies an object
type ObjectReference struct {
	Type *ObjectType
	Id   string
}

func NewObjectReference(objType *ObjectType, id string) (*ObjectReference, error) {
	if objType == nil {
		return nil, errors.New("type cannot be nil")
	}
	if id == "" {
		return nil, errors.New("id cannot be empty")
	}
	return &ObjectReference{Type: objType, Id: id}, nil
}

// SubjectReference represents a subject in a relationship
type SubjectReference struct {
	Relation *string          // Optional
	Subject  *ObjectReference
}

func NewSubjectReference(subject *ObjectReference, relation *string) (*SubjectReference, error) {
	if subject == nil {
		return nil, errors.New("subject cannot be nil")
	}
	return &SubjectReference{Subject: subject, Relation: relation}, nil
}

// Relationship represents a relationship tuple
type Relationship struct {
	Resource *ObjectReference
	Relation string
	Subject  *SubjectReference
}

func NewRelationship(resource *ObjectReference, relation string, subject *SubjectReference) (*Relationship, error) {
	if resource == nil {
		return nil, errors.New("resource cannot be nil")
	}
	if relation == "" {
		return nil, errors.New("relation cannot be empty")
	}
	if subject == nil {
		return nil, errors.New("subject cannot be nil")
	}
	return &Relationship{Resource: resource, Relation: relation, Subject: subject}, nil
}

// ConsistencyToken represents a consistency token for read-after-write
type ConsistencyToken struct {
	Token string
}

// Consistency specifies the consistency requirement for operations
type Consistency struct {
	MinimizeLatency bool
	AtLeastAsFresh  *ConsistencyToken
}

func NewConsistencyMinimizeLatency() *Consistency {
	return &Consistency{MinimizeLatency: true}
}

func NewConsistencyAtLeastAsFresh(token string) *Consistency {
	return &Consistency{AtLeastAsFresh: &ConsistencyToken{Token: token}}
}
