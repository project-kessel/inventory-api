package model

import (
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/google/uuid"
)

type ValidationError struct {
	Field   string
	Message string
}

func (e ValidationError) Error() string {
	return fmt.Sprintf("%s: %s", e.Field, e.Message)
}

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

var (
	ErrRequired    = errors.New("required field")
	ErrTooLong     = errors.New("exceeds maximum length")
	ErrTooSmall    = errors.New("below minimum value")
	ErrInvalidURL  = errors.New("invalid url")
	ErrInvalidUUID = errors.New("invalid uuid")
)

const (
	msgRequired    = "cannot be empty"
	msgTooLong     = "exceeds %d chars"
	msgTooSmall    = "must be >= %d"
	msgInvalidURL  = "invalid url: %v"
	msgInvalidUUID = "cannot be empty"
)

func validateRequired(field string, isValid bool) error {
	if isValid {
		return nil
	}
	return ValidationError{Field: field, Message: msgRequired}
}

func validateStringRequired(field, value string) error {
	return validateRequired(field, strings.TrimSpace(value) != "")
}

func validateUUIDRequired(field string, id uuid.UUID) error {
	if id == uuid.Nil {
		return ValidationError{Field: field, Message: msgInvalidUUID}
	}
	return nil
}

func validateMaxLength(field, value string, maxLength int) error {
	if len(value) > maxLength {
		return ValidationError{Field: field, Message: fmt.Sprintf(msgTooLong, maxLength)}
	}
	return nil
}

func validateMinValue(field string, value uint, minValue uint) error {
	if value < minValue {
		return ValidationError{Field: field, Message: fmt.Sprintf(msgTooSmall, minValue)}
	}
	return nil
}

func validateMinValueUint(field string, value uint, minValue uint) error {
	return validateMinValue(field, value, minValue)
}

func validateOptionalString(field string, value *string, maxLength int) error {
	if value == nil {
		return nil
	}
	return validateMaxLength(field, *value, maxLength)
}

func validateOptionalURL(field, value string, maxLength int) error {
	if value == "" {
		return nil
	}

	if len(value) > maxLength {
		return ValidationError{Field: field, Message: fmt.Sprintf(msgTooLong, maxLength)}
	}

	if _, err := url.Parse(value); err != nil {
		return ValidationError{Field: field, Message: fmt.Sprintf(msgInvalidURL, err)}
	}
	return nil
}

func aggregateErrors(errs ...error) error {
	var validationErrors []ValidationError
	for _, err := range errs {
		if err != nil {
			if ve, ok := err.(ValidationError); ok {
				validationErrors = append(validationErrors, ve)
			} else {
				validationErrors = append(validationErrors, ValidationError{
					Field:   "unknown",
					Message: err.Error(),
				})
			}
		}
	}

	if len(validationErrors) == 0 {
		return nil
	}

	if len(validationErrors) == 1 {
		return validationErrors[0]
	}

	var messages []string
	for _, ve := range validationErrors {
		messages = append(messages, ve.Error())
	}
	return errors.New(strings.Join(messages, "; "))
}
