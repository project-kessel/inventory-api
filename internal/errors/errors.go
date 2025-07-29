package errors

import (
	"strings"
)

// Aggregate represents a collection of errors that can be treated as a single error.
type Aggregate struct {
	Errors []error
}

// NewAggregate creates a new Aggregate error from a slice of errors.
func NewAggregate(errs []error) Aggregate {
	return Aggregate{errs}
}

// Error returns a string representation of all aggregated errors, joined by newlines.
func (a Aggregate) Error() string {
	var strs []string
	for _, e := range a.Errors {
		strs = append(strs, e.Error())
	}
	return strings.Join(strs, "\n")
}

// HttpError represents an HTTP error with status code and message.
type HttpError struct {
	Status  int    `json:"status"`
	Message string `json:"message"`
}
