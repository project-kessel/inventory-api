package model

import (
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
)

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

// Validation helper functions

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
func validateMinValue(field string, value, minValue int) error {
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
