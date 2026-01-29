package model

// SubjectReference represents a subject in an authorization check.
// It wraps a resource key (identifying the subject) with an optional relation.
type SubjectReference struct {
	subject  ReporterResourceKey
	relation *Relation // nil means no relation (direct subject reference)
}

// NewSubjectReference creates a new SubjectReference with a relation.
func NewSubjectReference(subject ReporterResourceKey, relation *Relation) SubjectReference {
	return SubjectReference{
		subject:  subject,
		relation: relation,
	}
}

// NewSubjectReferenceWithoutRelation creates a new SubjectReference without a relation.
func NewSubjectReferenceWithoutRelation(subject ReporterResourceKey) SubjectReference {
	return SubjectReference{
		subject:  subject,
		relation: nil,
	}
}

// Subject returns the resource key identifying the subject.
func (sr SubjectReference) Subject() ReporterResourceKey {
	return sr.subject
}

// Relation returns the optional relation on the subject (may be nil).
func (sr SubjectReference) Relation() *Relation {
	return sr.relation
}

// HasRelation returns true if this subject reference has a relation.
func (sr SubjectReference) HasRelation() bool {
	return sr.relation != nil
}
