package model

import (
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/google/uuid"
)

// =============================================================================
// Types and Data Structures
// =============================================================================

// ValidationError represents a domain validation error
type ValidationError struct {
	Field   string
	Message string
}

func (e ValidationError) Error() string {
	return fmt.Sprintf("%s: %s", e.Field, e.Message)
}

// =============================================================================
// Constants - Database Field Sizes
// =============================================================================

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

// =============================================================================
// Constants - Column Names
// =============================================================================

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

// =============================================================================
// Constants - Index Names and Database Types
// =============================================================================

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

// =============================================================================
// Validation - Sentinel Errors and Messages
// =============================================================================

// Sentinel errors for different validation failure types
var (
	ErrRequired    = errors.New("required field")
	ErrTooLong     = errors.New("exceeds maximum length")
	ErrTooSmall    = errors.New("below minimum value")
	ErrInvalidURL  = errors.New("invalid url")
	ErrInvalidUUID = errors.New("invalid uuid")
)

// Validation error messages as constants
const (
	msgRequired    = "cannot be empty"
	msgTooLong     = "exceeds %d chars"
	msgTooSmall    = "must be >= %d"
	msgInvalidURL  = "invalid url: %v"
	msgInvalidUUID = "cannot be empty"
)

// =============================================================================
// Validation Helper Functions
// =============================================================================

// validateRequired checks if a condition is met for required fields
func validateRequired(field string, isValid bool) error {
	if isValid {
		return nil
	}
	return ValidationError{Field: field, Message: msgRequired}
}

// validateStringRequired checks if a string field is not empty after trimming
func validateStringRequired(field, value string) error {
	return validateRequired(field, strings.TrimSpace(value) != "")
}

// validateUUIDRequired checks if a UUID is not nil
func validateUUIDRequired(field string, id uuid.UUID) error {
	if id == uuid.Nil {
		return ValidationError{Field: field, Message: msgInvalidUUID}
	}
	return nil
}

// validateMaxLength checks if a string doesn't exceed maximum length
func validateMaxLength(field, value string, maxLength int) error {
	if len(value) <= maxLength {
		return nil
	}
	return ValidationError{Field: field, Message: fmt.Sprintf(msgTooLong, maxLength)}
}

// validateMinValue checks if an integer meets minimum value requirement
func validateMinValue(field string, value, minValue uint) error {
	if value >= minValue {
		return nil
	}
	return ValidationError{Field: field, Message: fmt.Sprintf(msgTooSmall, minValue)}
}

// validateMinValueUint checks if a uint meets minimum value requirement
func validateMinValueUint(field string, value uint, minValue uint) error {
	if value >= minValue {
		return nil
	}
	return ValidationError{Field: field, Message: fmt.Sprintf(msgTooSmall, minValue)}
}

// validateOptionalURL validates a URL if it's provided (non-empty)
func validateOptionalURL(field, urlValue string, maxLength int) error {
	if urlValue == "" {
		return nil // Optional field
	}

	// Check length first
	if err := validateMaxLength(field, urlValue, maxLength); err != nil {
		return err
	}

	// Then validate URL format
	if err := validateURL(urlValue); err != nil {
		return ValidationError{Field: field, Message: fmt.Sprintf(msgInvalidURL, err)}
	}
	return nil
}

// validateOptionalString validates an optional string field for max length only
func validateOptionalString(field string, value *string, maxLength int) error {
	if value == nil || *value == "" {
		return nil // Optional field
	}
	return validateMaxLength(field, *value, maxLength)
}

// validateURL ensures a string is a valid absolute URL (scheme + host).
func validateURL(u string) error {
	parsed, err := url.ParseRequestURI(u)
	if err != nil {
		return fmt.Errorf("invalid url: %w", err)
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return fmt.Errorf("url must include scheme and host")
	}
	return nil
}

// aggregateErrors collects multiple validation errors into a single error
func aggregateErrors(errs ...error) error {
	var validationErrs []error
	for _, err := range errs {
		if err != nil {
			validationErrs = append(validationErrs, err)
		}
	}

	if len(validationErrs) == 0 {
		return nil
	}

	if len(validationErrs) == 1 {
		return validationErrs[0]
	}

	// For multiple errors, we can use the existing errors package
	return errors.Join(validationErrs...)
}

// =============================================================================
// GORM Tag Helper Functions
// =============================================================================

// These functions help generate consistent GORM struct tags using the defined constants

// buildGORMTag creates a GORM tag string from the provided options
func buildGORMTag(options ...string) string {
	return strings.Join(options, ";")
}

// sizeTag creates a size tag for GORM
func sizeTag(size int) string {
	return fmt.Sprintf("size:%d", size)
}

// columnTag creates a column tag for GORM
func columnTag(name string) string {
	return fmt.Sprintf("column:%s", name)
}

// typeTag creates a type tag for GORM
func typeTag(dbType string) string {
	return fmt.Sprintf("type:%s", dbType)
}

// checkTag creates a check constraint tag for GORM
func checkTag(constraint string) string {
	return fmt.Sprintf("check:%s", constraint)
}

// indexTag creates an index tag for GORM
func indexTag(name string, unique bool) string {
	if unique {
		return fmt.Sprintf("index:%s,unique", name)
	}
	return fmt.Sprintf("index:%s", name)
}

// primaryKeyTag creates a primary key tag
func primaryKeyTag() string {
	return "primaryKey"
}

// =============================================================================
// Common GORM Tag Builders
// =============================================================================

// StandardStringField creates a GORM tag for a standard string field
func StandardStringField(column string, size int) string {
	return buildGORMTag(sizeTag(size), columnTag(column))
}

// BigIntField creates a GORM tag for a bigint field with check constraint
func BigIntField(column string, checkConstraint string) string {
	return buildGORMTag(typeTag(DBTypeBigInt), columnTag(column), checkTag(checkConstraint))
}

// PrimaryKeyField creates a GORM tag for a primary key field
func PrimaryKeyField(column string, dbType string) string {
	return buildGORMTag(typeTag(dbType), columnTag(column), primaryKeyTag())
}

// UniqueIndexField creates a GORM tag for a field that's part of a unique index
func UniqueIndexField(column string, size int, indexName string) string {
	return buildGORMTag(sizeTag(size), columnTag(column), indexTag(indexName, true))
}
