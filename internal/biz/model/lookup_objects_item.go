package model

// LookupObjectsItem represents a single object from a lookup stream.
type LookupObjectsItem struct {
	object            ResourceReference
	continuationToken ContinuationToken
}

func NewLookupObjectsItem(object ResourceReference, continuationToken ContinuationToken) LookupObjectsItem {
	return LookupObjectsItem{object: object, continuationToken: continuationToken}
}

func (i LookupObjectsItem) Object() ResourceReference            { return i.object }
func (i LookupObjectsItem) ContinuationToken() ContinuationToken { return i.continuationToken }
