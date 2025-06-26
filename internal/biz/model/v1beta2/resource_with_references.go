package v1beta2

// ResourceWithReferences represents the complete aggregate
type ResourceWithReferences struct {
	Resource                 *Resource
	RepresentationReferences []*RepresentationReference
}
