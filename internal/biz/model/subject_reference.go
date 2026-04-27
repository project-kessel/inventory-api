package model

// SubjectReference represents a subject in an authorization check.
// It wraps a ResourceReference (identifying the subject) with an optional relation.
type SubjectReference struct {
	resource ResourceReference
	relation *Relation // nil means no relation (direct subject reference)
}

// NewSubjectReference creates a new SubjectReference with a relation.
func NewSubjectReference(resource ResourceReference, relation *Relation) SubjectReference {
	return SubjectReference{
		resource: resource,
		relation: relation,
	}
}

// NewSubjectReferenceWithoutRelation creates a new SubjectReference without a relation.
func NewSubjectReferenceWithoutRelation(resource ResourceReference) SubjectReference {
	return SubjectReference{
		resource: resource,
		relation: nil,
	}
}

// Resource returns the ResourceReference identifying the subject.
func (sr SubjectReference) Resource() ResourceReference {
	return sr.resource
}

// Relation returns the optional relation on the subject (may be nil).
func (sr SubjectReference) Relation() *Relation {
	return sr.relation
}

// HasRelation returns true if this subject reference has a relation.
func (sr SubjectReference) HasRelation() bool {
	return sr.relation != nil
}
