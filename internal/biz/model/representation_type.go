package model

import "fmt"

// RepresentationType identifies a kind of resource, optionally scoped to a reporter.
// Matches Inventory v1beta2 RepresentationType proto semantics.
type RepresentationType struct {
	resourceType ResourceType  // required
	reporterType *ReporterType // optional
}

func NewRepresentationType(resourceType ResourceType, reporterType *ReporterType) RepresentationType {
	return RepresentationType{
		resourceType: resourceType,
		reporterType: reporterType,
	}
}

func NewRepresentationTypeRequired(resourceType ResourceType, reporterType ReporterType) RepresentationType {
	return RepresentationType{
		resourceType: resourceType,
		reporterType: &reporterType,
	}
}

func (rt RepresentationType) ResourceType() ResourceType  { return rt.resourceType }
func (rt RepresentationType) ReporterType() *ReporterType { return rt.reporterType }
func (rt RepresentationType) HasReporterType() bool       { return rt.reporterType != nil }

func (rt RepresentationType) RequireReporterType() (ReporterType, error) {
	if rt.reporterType == nil {
		return ReporterType(""), fmt.Errorf("RepresentationType: reporter type is required but not set")
	}
	return *rt.reporterType, nil
}
