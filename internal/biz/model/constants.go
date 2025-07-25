package model

// Database field size constants
// These constants define the maximum lengths for various database fields
// to ensure consistency between GORM struct tags and validation logic.
const (
	// Standard field sizes
	MaxFieldSize128 = 128 // For most string fields like IDs, types, etc.
	MaxFieldSize256 = 256 // For longer fields like reporter instance IDs
	MaxFieldSize512 = 512 // For URL fields like APIHref, ConsoleHref

	// Specific field size constants for better readability
	MaxLocalResourceIDLength    = MaxFieldSize128
	MaxReporterTypeLength       = MaxFieldSize128
	MaxResourceTypeLength       = MaxFieldSize128
	MaxReporterInstanceIDLength = MaxFieldSize256
	MaxReporterVersionLength    = MaxFieldSize128
	MaxAPIHrefLength            = MaxFieldSize512
	MaxConsoleHrefLength        = MaxFieldSize512

	// Minimum values for validation
	MinVersionValue    = 0 // Version can be zero or positive (>= 0)
	MinGenerationValue = 0 // Generation can be zero or positive (>= 0)
	MinCommonVersion   = 0 // CommonVersion can be zero or positive (>= 0)
)

// Column name constants
const (
	// CommonRepresentation columns
	ColumnResourceID                 = "id"
	ColumnVersion                    = "version"
	ColumnReportedByReporterType     = "reported_by_reporter_type"
	ColumnReportedByReporterInstance = "reported_by_reporter_instance"
	ColumnData                       = "data"

	// ReporterResource columns (identifying)
	ColumnReporterResourceID = "id"
	ColumnLocalResourceID    = "local_resource_id"
	ColumnRRResourceType     = "resource_type"
	ColumnReporterType       = "reporter_type"
	ColumnReporterInstanceID = "reporter_instance_id"

	// ReporterResource extra columns
	ColumnRRAPIHref     = "api_href"
	ColumnRRConsoleHref = "console_href"
	ColumnRRGeneration  = "generation"
	ColumnRRVersion     = "version"
	ColumnRRTombstone   = "tombstone"
	ColumnRRResourceFK  = "resource_id"

	// ReporterRepresentation columns
	ColumnRepReporterResourceID = "reporter_resource_id"
	ColumnRepVersion            = "version"
	ColumnRepGeneration         = "generation"
	ColumnCommonVersion         = "common_version"
	ColumnTombstone             = "tombstone"
	ColumnReporterVersion       = "reporter_version"
)

// Index names
const (
	ReporterResourceKeyIdx    = "reporter_resource_key_idx"
	ReporterResourceSearchIdx = "reporter_resource_search_idx"
)

// Database type constants
const (
	DBTypeText   = "text"
	DBTypeBigInt = "bigint"
	DBTypeJSONB  = "jsonb"
)
