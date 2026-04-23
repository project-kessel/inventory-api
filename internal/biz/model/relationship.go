package model

// Relationship represents a query concept in the Relations domain:
// "does this Object have this Relation to this Subject?"
// May be backed by a stored tuple or derived via schema rules.
type Relationship struct {
	object   ResourceReference
	relation Relation
	subject  SubjectReference
}

func NewRelationship(object ResourceReference, relation Relation, subject SubjectReference) Relationship {
	return Relationship{
		object:   object,
		relation: relation,
		subject:  subject,
	}
}

func (r Relationship) Object() ResourceReference { return r.object }
func (r Relationship) Relation() Relation        { return r.relation }
func (r Relationship) Subject() SubjectReference { return r.subject }
