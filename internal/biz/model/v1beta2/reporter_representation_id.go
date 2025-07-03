package v1beta2

// ReporterRepresentationId represents how a service provider uniquely identifies
// a resource from their perspective. This is the key that reporters use to
// identify their resource and find all related representation references.
type ReporterRepresentationId struct {
	LocalResourceID    string // The ID the reporter uses for this resource
	ReporterType       string // What kind of reporter (e.g., "hbi", "acm")
	ResourceType       string // What kind of resource (e.g., "host", "cluster")
	ReporterInstanceID string // Which instance of the reporter
}
