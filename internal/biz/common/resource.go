package common

import (
	"time"
)

type Metadata struct {
	// Kessel Asset Inventory generated identifier.
	ID int64 `gorm:"primaryKey"`

	CreatedAt time.Time
	UpdatedAt time.Time

	// The type of the Resource.
	ResourceType string

	// Date and time when the inventory item was first reported.
	FirstReported time.Time

	// Identity of the reporter that first reported this item.
	FirstReportedBy string

	// Date and time when the inventory item was last updated.
	LastReported time.Time

	// Identity of the reporter that last reported on this item.
	LastReportedBy string

	// The workspace in which this resource is a member for access control.  A resource can only be a member
	// of one workspace.
	Workspace string

	// Write only reporter specific data
	Reporters []*Reporter

	Tags []*Tag
}

type Reporter struct {
	// This is necessary to satisfy gorm so the collection in the Resource model works.
	MetadataID int64 `json:"-"`

	// ReporterID should be populated from the Identity of the caller.  e.g. if this is an ACM reporter, *which* ACM
	// instance is it?
	ReporterID string `gorm:"primaryKey"`

	// This is the type of the Data blob below.  It specifies whether this is an OCM cluster, an ACM cluster,
	// etc.  It seems reasonable to infer the value from the caller's identity data, but it's not clear that's
	// *always* the case.  So, allow it to be passed explicitly and then log a warning or something if the value
	// doesn't match the inferred type.
	ReporterType string `gorm:"primaryKey"`

	// LocalResourceId is the identifier assigned to the resource within the reporter's system.
	LocalResourceId string `gorm:"primaryKey"`

	// The version of the reporter.
	ReporterVersion string

	// pointers to where to access the resource
	ConsoleHref string
	ApiHref     string
}

type Tag struct {
	ID         int64 `gorm:"primaryKey"`
	MetadataID int64

	Key   string
	Value string
}
