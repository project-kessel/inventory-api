package model

import (
	"strings"
)

type RelationsTuple struct {
	resource string
	relation string
	subject  string
}

func NewRelationsTuple(resource, relation, subject string) RelationsTuple {
	return RelationsTuple{
		resource: strings.ToLower(resource),
		relation: strings.ToLower(relation),
		subject:  strings.ToLower(subject),
	}
}

func (rt RelationsTuple) Resource() string {
	return rt.resource
}

func (rt RelationsTuple) Relation() string {
	return rt.relation
}

func (rt RelationsTuple) Subject() string {
	return rt.subject
}
