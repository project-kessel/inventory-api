package model

// LookupSubjectsItem represents a single subject from a lookup stream.
type LookupSubjectsItem struct {
	subject           SubjectReference
	continuationToken string
}

func NewLookupSubjectsItem(subject SubjectReference, continuationToken string) LookupSubjectsItem {
	return LookupSubjectsItem{subject: subject, continuationToken: continuationToken}
}

func (i LookupSubjectsItem) Subject() SubjectReference { return i.subject }
func (i LookupSubjectsItem) ContinuationToken() string { return i.continuationToken }
