package model

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
