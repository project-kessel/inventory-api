package common

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

type Metadata struct {
	// Kessel Asset Inventory generated identifier.
	ID int64 `gorm:"primaryKey" json:"id"`

	// These fields / names have special meaning to gorm, which will handle them automatically.
	CreatedAt time.Time `json:"-"`
	UpdatedAt time.Time `json:"last_reported"`

	// The type of the Resource.
	ResourceType string `json:"resource_type"`

	// Identity of the reporter that first reported this item.
	FirstReportedBy string `json:"-"`

	// Identity of the reporter that last reported on this item.
	LastReportedBy string `json:"-"`

	// The workspace in which this resource is a member for access control.  A resource can only be a member
	// of one workspace.
	Workspace string `json:"workspace"`

	// Write only reporter specific data
	Reporters []*Reporter `json:"-"`

	Labels []*Label `json:"labels"`
}

type Reporter struct {
	// This is necessary to satisfy gorm so the collection in the Resource model works.
	MetadataID int64 `json:"-"`

	// ReporterID should be populated from the Identity of the caller.  e.g. if this is an ACM reporter, *which* ACM
	// instance is it?
	ReporterID string `gorm:"primaryKey" json:"reporter_instance_id"`

	// This is the type of the Data blob below.  It specifies whether this is an OCM cluster, an ACM cluster,
	// etc.  It seems reasonable to infer the value from the caller's identity data, but it's not clear that's
	// *always* the case.  So, allow it to be passed explicitly and then log a warning or something if the value
	// doesn't match the inferred type.
	ReporterType string `gorm:"primaryKey" json:"reporter_type"`

	// Date and time when the inventory item was first reported.
	CreatedAt time.Time `json:"-"`

	// Date and time when the inventory item was last updated.
	UpdatedAt time.Time `json:"last_reported"`

	// LocalResourceId is the identifier assigned to the resource within the reporter's system.
	LocalResourceId string `gorm:"primaryKey" json:"local_resource_id"`

	// The version of the reporter.
	ReporterVersion string `json:"reporter_version"`

	// pointers to where to access the resource
	ConsoleHref string `json:"console_href"`
	ApiHref     string `json:"api_href"`
}

type Label struct {
	ID         int64 `gorm:"primaryKey" json:"-"`
	MetadataID int64 `json:"-"`

	Key   string `json:"key"`
	Value string `json:"value"`
}

type RelationshipMetadata struct {
	ID int64 `gorm:"primaryKey" json:"id"`
	// These fields / names have special meaning to gorm, which will handle them automatically.
	CreatedAt time.Time `json:"-"`
	UpdatedAt time.Time `json:"last_reported"`

	// These fields / names have special meaning to gorm, which will handle them automatically.
	RelationshipType string `json:"relationship_type"`

	// Identity of the reporter that first reported this item.
	FirstReportedBy string `json:"-"`

	// Identity of the reporter that last reported on this item.
	LastReportedBy string `json:"-"`

	// Write only reporter specific data
	Reporters []*RelationshipReporter `json:"-"`
}

type RelationshipReporter struct {
	// This is necessary to satisfy gorm so the collection in the Resource model works.
	RelationshipMetadataID int64 `json:"-"`

	// ReporterID should be populated from the Identity of the caller.  e.g. if this is an ACM reporter, *which* ACM
	// instance is it?
	ReporterID string `gorm:"primaryKey" json:"reporter_instance_id"`

	// This is the type of the Data blob below.  It specifies whether this is an OCM cluster, an ACM cluster,
	// etc.  It seems reasonable to infer the value from the caller's identity data, but it's not clear that's
	// *always* the case.  So, allow it to be passed explicitly and then log a warning or something if the value
	// doesn't match the inferred type.
	ReporterType string `gorm:"primaryKey" json:"reporter_type"`

	// Date and time when the inventory item was first reported.
	CreatedAt time.Time `json:"-"`

	// Date and time when the inventory item was last updated.
	UpdatedAt time.Time `json:"last_reported"`

	// SubjectLocalResourceId is the identifier assigned to the resource within the reporter's system.
	SubjectLocalResourceId string `gorm:"primaryKey" json:"subject_local_resource_id"`

	// ObjectLocalResourceId is the identifier assigned to the resource within the reporter's system.
	ObjectLocalResourceId string `gorm:"primaryKey" json:"object_local_resource_id"`

	// The version of the reporter.
	ReporterVersion string `json:"reporter_version"`
}

func Unmarshal(in interface{}, out interface{}) error {
	bytes, ok := in.([]byte)
	if !ok {
		return errors.New(fmt.Sprint("Failed to unmarshal JSONB value:", in))
	}

	err := json.Unmarshal(bytes, &out)
	return err
}

func Marshal(in interface{}) (driver.Value, error) {
	return json.Marshal(in)
}
