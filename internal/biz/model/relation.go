package model

import (
	"fmt"
	"strings"
)

// Relation represents a relation name in the authorization model.
type Relation string

// NewRelation creates a new Relation from a string.
// The relation is validated: it must be non-empty after trimming and contain no spaces.
func NewRelation(relation string) (Relation, error) {
	relation = strings.TrimSpace(relation)
	if relation == "" {
		return Relation(""), fmt.Errorf("%w: Relation", ErrEmpty)
	}
	if strings.ContainsAny(relation, " \t\n\r") {
		return Relation(""), fmt.Errorf("%w: Relation contains whitespace", ErrInvalidData)
	}
	return Relation(relation), nil
}

// String returns the string representation of the Relation.
func (r Relation) String() string {
	return string(r)
}

// Serialize returns the string value for storage or transmission.
func (r Relation) Serialize() string {
	return string(r)
}

// DeserializeRelation creates a Relation from a stored value without validation.
func DeserializeRelation(value string) Relation {
	return Relation(value)
}
