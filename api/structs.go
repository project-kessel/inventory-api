package api

import (
	"time"

	"github.com/project-kessel/inventory-api/internal"
)

// JsonObject represents a generic JSON object as a map of string keys to interface{} values.
// This is an alias to internal.JsonObject for consistency across the codebase.
type JsonObject = internal.JsonObject

// Event represents a CloudEvent structure for inventory system events.
// Type taken from: https://github.com/project-kessel/inventory-api/blob/main/internal/eventing/api/event.go
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

// ResourceData contains the data payload for resource-related events (v1beta1).
type ResourceData struct {
	Metadata     ResourceMetadata `json:"metadata"`
	ReporterData ResourceReporter `json:"reporter_data"`
	ResourceData JsonObject       `json:"resource_data,omitempty"`
}

// RelationshipData contains the data payload for relationship-related events (v1beta1).
type RelationshipData struct {
	Metadata     RelationshipMetadata `json:"metadata"`
	ReporterData RelationshipReporter `json:"reporter_data"`
	ResourceData JsonObject           `json:"resource_data,omitempty"`
}

// ResourceMetadata contains metadata information for inventory resources (v1beta1).
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

// ResourceLabel represents a key-value label associated with a resource (v1beta1).
type ResourceLabel struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// ResourceReporter contains information about the system that reported the resource (v1beta1).
type ResourceReporter struct {
	ReporterInstanceId string `json:"reporter_instance_id"`
	ReporterType       string `json:"reporter_type"`
	ConsoleHref        string `json:"console_href"`
	ApiHref            string `json:"api_href"`
	LocalResourceId    string `json:"local_resource_id"`
	ReporterVersion    string `json:"reporter_version"`
}

// RelationshipMetadata contains metadata information for inventory relationships (v1beta1).
type RelationshipMetadata struct {
	Id               string     `json:"id"`
	RelationshipType string     `json:"relationship_type"`
	CreatedAt        *time.Time `json:"created_at,omitempty"`
	UpdatedAt        *time.Time `json:"updated_at,omitempty"`
	DeletedAt        *time.Time `json:"deleted_at,omitempty"`
}

// RelationshipReporter contains information about the system that reported the relationship (v1beta1).
type RelationshipReporter struct {
	ReporterType           string `json:"reporter_type"`
	SubjectLocalResourceId string `json:"subject_local_resource_id"`
	ObjectLocalResourceId  string `json:"object_local_resource_id"`
	ReporterVersion        string `json:"reporter_version"`
	ReporterInstanceId     string `json:"reporter_instance_id"`
}
