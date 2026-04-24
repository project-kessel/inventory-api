package model

// LookupObjectsItem represents a single object from a lookup stream.
type LookupObjectsItem struct {
	object            ResourceReference
	continuationToken string
}

func NewLookupObjectsItem(object ResourceReference, continuationToken string) LookupObjectsItem {
	return LookupObjectsItem{object: object, continuationToken: continuationToken}
}

func (i LookupObjectsItem) Object() ResourceReference { return i.object }
func (i LookupObjectsItem) ContinuationToken() string { return i.continuationToken }
