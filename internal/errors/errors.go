package errors

import (
	"strings"
)

type Aggregate struct {
	Errors []error
}

func NewAggregate(errs []error) Aggregate {
	return Aggregate{errs}
}

func (a Aggregate) Error() string {
	var strs []string
	for _, e := range a.Errors {
		strs = append(strs, e.Error())
	}
	return strings.Join(strs, "\n")
}

type HttpError struct {
	Status  int    `json:"status"`
	Message string `json:"messgae"`
}
