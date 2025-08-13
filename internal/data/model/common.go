package model

// Field size constraints for GORM models
const (
	MaxFieldSize128  = 128
	MaxFieldSize256  = 256
	MaxFieldSize512  = 512
	MaxFieldSize1024 = 1024

	MaxLocalResourceIDLength    = MaxFieldSize128
	MaxReporterTypeLength       = MaxFieldSize128
	MaxResourceTypeLength       = MaxFieldSize128
	MaxReporterInstanceIDLength = MaxFieldSize256
	MaxReporterVersionLength    = MaxFieldSize128
	MaxAPIHrefLength            = MaxFieldSize512
	MaxConsoleHrefLength        = MaxFieldSize512
	MaxConsistencyTokenLength   = MaxFieldSize1024

	MinVersionValue    = 0
	MinGenerationValue = 0
	MinCommonVersion   = 0
)

const (
	ColumnResourceID                 = "id"
	ColumnVersion                    = "version"
	ColumnReportedByReporterType     = "reported_by_reporter_type"
	ColumnReportedByReporterInstance = "reported_by_reporter_instance"
	ColumnData                       = "data"

	ColumnReporterResourceID = "id"
	ColumnLocalResourceID    = "local_resource_id"
	ColumnRRResourceType     = "resource_type"
	ColumnReporterType       = "reporter_type"
	ColumnReporterInstanceID = "reporter_instance_id"

	ColumnRRAPIHref     = "api_href"
	ColumnRRConsoleHref = "console_href"
	ColumnRRGeneration  = "generation"
	ColumnRRVersion     = "version"
	ColumnRRTombstone   = "tombstone"
	ColumnRRResourceFK  = "resource_id"

	ColumnRepReporterResourceID = "reporter_resource_id"
	ColumnRepVersion            = "version"
	ColumnRepGeneration         = "generation"
	ColumnCommonVersion         = "common_version"
	ColumnTombstone             = "tombstone"
	ColumnReporterVersion       = "reporter_version"
)

const (
	ReporterResourceKeyIdx    = "reporter_resource_key_idx"
	ReporterResourceSearchIdx = "reporter_resource_search_idx"
)

const (
	DBTypeText   = "text"
	DBTypeBigInt = "bigint"
	DBTypeJSONB  = "jsonb"
)
