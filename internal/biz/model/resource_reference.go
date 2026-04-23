package model

import "fmt"

// ResourceReference identifies a specific resource to check access for
type ResourceReference struct {
	resourceType ResourceType       // required
	resourceId   LocalResourceId    // required
	reporter     *ReporterReference // optional
}

func NewResourceReference(resourceType ResourceType, resourceId LocalResourceId, reporter *ReporterReference) ResourceReference {
	return ResourceReference{
		resourceType: resourceType,
		resourceId:   resourceId,
		reporter:     reporter,
	}
}

func (r ResourceReference) ResourceType() ResourceType   { return r.resourceType }
func (r ResourceReference) ResourceId() LocalResourceId  { return r.resourceId }
func (r ResourceReference) Reporter() *ReporterReference { return r.reporter }
func (r ResourceReference) HasReporter() bool            { return r.reporter != nil }

// ReporterReference identifies a reporter within a ResourceReference.
type ReporterReference struct {
	reporterType ReporterType        // required
	instanceId   *ReporterInstanceId // optional
}

func NewReporterReference(reporterType ReporterType, instanceId *ReporterInstanceId) ReporterReference {
	return ReporterReference{
		reporterType: reporterType,
		instanceId:   instanceId,
	}
}

func (r ReporterReference) ReporterType() ReporterType      { return r.reporterType }
func (r ReporterReference) InstanceId() *ReporterInstanceId { return r.instanceId }
func (r ReporterReference) HasInstanceId() bool             { return r.instanceId != nil }

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
