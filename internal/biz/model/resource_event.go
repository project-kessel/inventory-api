package model

type ResourceEvent interface {
	Id() ResourceId
	ResourceType() string
	ReporterType() string
	ReporterInstanceId() string
	LocalResourceId() string
	WorkspaceId() string
	CommonVersion() Version
	ReporterResourceKey() (ReporterResourceKey, error)
}
