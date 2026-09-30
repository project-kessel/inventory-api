package model

type ResourceEvent interface {
	Id() ResourceId
	ResourceType() ResourceType
	ReporterType() ReporterType
	ReporterInstanceId() string
	LocalResourceId() string
	WorkspaceId() *string
	CurrentCommonVersion() *Version
	CurrentReporterRepresentationVersion() *Version
	ReporterResourceKey() ReporterResourceKey
}
