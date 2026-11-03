package model

// RepresentationVersions encapsulates the version coordinates for fetching representations
// from both the common and reporter streams. This value object groups related version
// information that travels together through the fetch pipeline.
type RepresentationVersions struct {
	commonVersion      *Version
	reporterVersion    *Version
	reporterGeneration *Generation
}

// NewRepresentationVersions creates a new RepresentationVersions value object.
// At least one version (common or reporter) must be provided.
func NewRepresentationVersions(
	commonVersion *Version,
	reporterVersion *Version,
	reporterGeneration *Generation,
) RepresentationVersions {
	return RepresentationVersions{
		commonVersion:      commonVersion,
		reporterVersion:    reporterVersion,
		reporterGeneration: reporterGeneration,
	}
}

// CommonVersion returns the common representation version, or nil if the common stream did not advance.
func (rv RepresentationVersions) CommonVersion() *Version {
	return rv.commonVersion
}

// ReporterVersion returns the reporter representation version, or nil if the reporter stream did not advance.
func (rv RepresentationVersions) ReporterVersion() *Version {
	return rv.reporterVersion
}

// ReporterGeneration returns the reporter generation, or nil if the reporter stream did not advance.
func (rv RepresentationVersions) ReporterGeneration() *Generation {
	return rv.reporterGeneration
}

// HasReporterCoordinates reports whether the reporter stream is fully identified.
// Events produced before reporter generations existed carry a reporter version but
// no generation; those must be handled through the common-only path, because a
// missing generation cannot be interpreted as generation zero.
func (rv RepresentationVersions) HasReporterCoordinates() bool {
	return rv.reporterVersion != nil && rv.reporterGeneration != nil
}

// NewRepresentationVersionsFromTupleEvent constructs RepresentationVersions from a TupleEvent.
// This is a convenience constructor for the consumer layer.
func NewRepresentationVersionsFromTupleEvent(tupleEvent TupleEvent) RepresentationVersions {
	return NewRepresentationVersions(
		tupleEvent.CommonVersion(),
		tupleEvent.ReporterRepresentationVersion(),
		tupleEvent.ReporterGeneration(),
	)
}
