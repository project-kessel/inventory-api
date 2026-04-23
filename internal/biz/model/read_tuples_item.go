package model

// ReadTuplesItem represents a single tuple returned from a ReadTuples stream.
type ReadTuplesItem struct {
	object            ResourceReference
	relation          Relation
	subject           SubjectReference
	continuationToken string
	consistencyToken  ConsistencyToken
}

func NewReadTuplesItem(object ResourceReference, relation Relation, subject SubjectReference, continuationToken string, consistencyToken ConsistencyToken) ReadTuplesItem {
	return ReadTuplesItem{
		object:            object,
		relation:          relation,
		subject:           subject,
		continuationToken: continuationToken,
		consistencyToken:  consistencyToken,
	}
}

func (i ReadTuplesItem) Object() ResourceReference       { return i.object }
func (i ReadTuplesItem) Relation() Relation              { return i.relation }
func (i ReadTuplesItem) Subject() SubjectReference       { return i.subject }
func (i ReadTuplesItem) ContinuationToken() string       { return i.continuationToken }
func (i ReadTuplesItem) ConsistencyToken() ConsistencyToken { return i.consistencyToken }
