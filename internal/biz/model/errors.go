package model

import (
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/google/uuid"
)

// Validation sentinel errors
var (
	ErrEmpty       = errors.New("cannot be empty")
	ErrTooLong     = errors.New("exceeds maximum length")
	ErrTooSmall    = errors.New("below minimum value")
	ErrInvalidURL  = errors.New("invalid url")
	ErrInvalidUUID = errors.New("invalid uuid")
)

// Domain sentinel errors - business rule violations
var (
	ErrVersionConflict   = errors.New("optimistic concurrency failure")
	ErrReporterDuplicate = errors.New("reporter already exists for resource")
	ErrResourceNotFound  = errors.New("resource not found")
	ErrInvalidData       = errors.New("invalid data structure")
	ErrEmptyReporterList = errors.New("must have at least one reporter resource")
)

// Service-level sentinel errors - operation failures
var (
	// ErrResourceAlreadyExists indicates the resource already exists when creating.
	ErrResourceAlreadyExists = errors.New("resource already exists")
	// ErrInventoryIdMismatch indicates the inventory ID in the request doesn't match the existing resource.
	ErrInventoryIdMismatch = errors.New("resource inventory id mismatch")
	// ErrDatabaseError indicates a database operation failure.
	ErrDatabaseError = errors.New("database error")
)

// ValidationError represents a field validation error with context
type ValidationError struct {
	Field   string
	Message string
}

func (e ValidationError) Error() string {
	return fmt.Sprintf("%s: %s", e.Field, e.Message)
}

// Validation helper functions that return sentinel errors

// ValidateRequired validates that a condition is true
func ValidateRequired(field string, isValid bool) error {
	if isValid {
		return nil
	}
	return fmt.Errorf("%w: %s", ErrEmpty, field)
}

// ValidateStringRequired validates that a string is not empty or whitespace
func ValidateStringRequired(field, value string) error {
	return ValidateRequired(field, strings.TrimSpace(value) != "")
}

// ValidateUUIDRequired validates that a UUID is not nil
func ValidateUUIDRequired(field string, id uuid.UUID) error {
	if id == uuid.Nil {
		return fmt.Errorf("%w: %s", ErrInvalidUUID, field)
	}
	return nil
}

// ValidateMaxLength validates that a string does not exceed maximum length
func ValidateMaxLength(field, value string, maxLength int) error {
	if len(value) > maxLength {
		return fmt.Errorf("%w: %s (max %d)", ErrTooLong, field, maxLength)
	}
	return nil
}

// ValidateMinValue validates that a uint value is not below minimum
func ValidateMinValue(field string, value uint, minValue uint) error {
	if value < minValue {
		return fmt.Errorf("%w: %s (min %d)", ErrTooSmall, field, minValue)
	}
	return nil
}

// ValidateMinValueUint is an alias for ValidateMinValue for backward compatibility
func ValidateMinValueUint(field string, value uint, minValue uint) error {
	return ValidateMinValue(field, value, minValue)
}

// ValidateOptionalString validates an optional string field
func ValidateOptionalString(field string, value *string, maxLength int) error {
	if value == nil {
		return nil
	}
	return ValidateMaxLength(field, *value, maxLength)
}

// ValidateOptionalURL validates an optional URL field
func ValidateOptionalURL(field, value string, maxLength int) error {
	if value == "" {
		return nil
	}

	if len(value) > maxLength {
		return fmt.Errorf("%w: %s (max %d)", ErrTooLong, field, maxLength)
	}

	if _, err := url.Parse(value); err != nil {
		return fmt.Errorf("%w: %s (%v)", ErrInvalidURL, field, err)
	}
	return nil
}

// AggregateErrors combines multiple validation errors into a single error
func AggregateErrors(errs ...error) error {
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
