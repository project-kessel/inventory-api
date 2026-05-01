package model

// LookupSubjectsItem represents a single subject from a lookup stream.
type LookupSubjectsItem struct {
	subject           SubjectReference
	continuationToken ContinuationToken
}

func NewLookupSubjectsItem(subject SubjectReference, continuationToken ContinuationToken) LookupSubjectsItem {
	return LookupSubjectsItem{subject: subject, continuationToken: continuationToken}
}

func (i LookupSubjectsItem) Subject() SubjectReference            { return i.subject }
func (i LookupSubjectsItem) ContinuationToken() ContinuationToken { return i.continuationToken }
