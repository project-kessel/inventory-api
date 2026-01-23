package api

import (
	"time"

	"github.com/project-kessel/inventory-api/internal"
)

// Event represents a CloudEvent structure used for inventory system events.
// Todo: get rid of this Event and have an Event (as event output) with all the assignments going on the New* functions
type Event struct {
	Specversion     string      `json:"specversion"`
	Type            string      `json:"type"`
	Source          string      `json:"source"`
	Id              string      `json:"id"`
	Subject         string      `json:"subject"`
	Time            time.Time   `json:"time"`
	DataContentType string      `json:"datacontenttype"`
	Data            interface{} `json:"data"`
}

// ResourceData contains the data payload for resource-related events.
type ResourceData struct {
	Metadata     ResourceMetadata    `json:"metadata"`
	ReporterData ResourceReporter    `json:"reporter_data"`
	ResourceData internal.JsonObject `json:"resource_data,omitempty"`
}

// ResourceMetadata contains metadata information for inventory resources.
type ResourceMetadata struct {
	Id           string          `json:"id"`
	ResourceType string          `json:"resource_type"`
	OrgId        string          `json:"org_id"`
	CreatedAt    *time.Time      `json:"created_at,omitempty"`
	UpdatedAt    *time.Time      `json:"updated_at,omitempty"`
	DeletedAt    *time.Time      `json:"deleted_at,omitempty"`
	WorkspaceId  string          `json:"workspace_id"`
	Labels       []ResourceLabel `json:"labels,omitempty"`
}

// ResourceLabel represents a key-value label associated with a resource.
type ResourceLabel struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// ResourceReporter contains information about the system that reported the resource.
type ResourceReporter struct {
	ReporterInstanceId string `json:"reporter_instance_id"`
	ReporterType       string `json:"reporter_type"`
	ConsoleHref        string `json:"console_href"`
	ApiHref            string `json:"api_href"`
	LocalResourceId    string `json:"local_resource_id"`
	ReporterVersion    string `json:"reporter_version"`
}
