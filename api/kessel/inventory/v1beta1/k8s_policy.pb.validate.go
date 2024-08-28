// Code generated by protoc-gen-validate. DO NOT EDIT.
// source: kessel/inventory/v1beta1/k8s_policy.proto

package v1beta1

import (
	"bytes"
	"errors"
	"fmt"
	"net"
	"net/mail"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"google.golang.org/protobuf/types/known/anypb"
)

// ensure the imports are used
var (
	_ = bytes.MinRead
	_ = errors.New("")
	_ = fmt.Print
	_ = utf8.UTFMax
	_ = (*regexp.Regexp)(nil)
	_ = (*strings.Reader)(nil)
	_ = net.IPv4len
	_ = time.Duration(0)
	_ = (*url.URL)(nil)
	_ = (*mail.Address)(nil)
	_ = anypb.Any{}
	_ = sort.Sort
)

// Validate checks the field values on K8SPolicy with the rules defined in the
// proto definition for this message. If any rules are violated, the first
// error encountered is returned, or nil if there are no violations.
func (m *K8SPolicy) Validate() error {
	return m.validate(false)
}

// ValidateAll checks the field values on K8SPolicy with the rules defined in
// the proto definition for this message. If any rules are violated, the
// result is a list of violation errors wrapped in K8SPolicyMultiError, or nil
// if none found.
func (m *K8SPolicy) ValidateAll() error {
	return m.validate(true)
}

func (m *K8SPolicy) validate(all bool) error {
	if m == nil {
		return nil
	}

	var errors []error

	if all {
		switch v := interface{}(m.GetMetadata()).(type) {
		case interface{ ValidateAll() error }:
			if err := v.ValidateAll(); err != nil {
				errors = append(errors, K8SPolicyValidationError{
					field:  "Metadata",
					reason: "embedded message failed validation",
					cause:  err,
				})
			}
		case interface{ Validate() error }:
			if err := v.Validate(); err != nil {
				errors = append(errors, K8SPolicyValidationError{
					field:  "Metadata",
					reason: "embedded message failed validation",
					cause:  err,
				})
			}
		}
	} else if v, ok := interface{}(m.GetMetadata()).(interface{ Validate() error }); ok {
		if err := v.Validate(); err != nil {
			return K8SPolicyValidationError{
				field:  "Metadata",
				reason: "embedded message failed validation",
				cause:  err,
			}
		}
	}

	if m.GetReporterData() == nil {
		err := K8SPolicyValidationError{
			field:  "ReporterData",
			reason: "value is required",
		}
		if !all {
			return err
		}
		errors = append(errors, err)
	}

	if all {
		switch v := interface{}(m.GetReporterData()).(type) {
		case interface{ ValidateAll() error }:
			if err := v.ValidateAll(); err != nil {
				errors = append(errors, K8SPolicyValidationError{
					field:  "ReporterData",
					reason: "embedded message failed validation",
					cause:  err,
				})
			}
		case interface{ Validate() error }:
			if err := v.Validate(); err != nil {
				errors = append(errors, K8SPolicyValidationError{
					field:  "ReporterData",
					reason: "embedded message failed validation",
					cause:  err,
				})
			}
		}
	} else if v, ok := interface{}(m.GetReporterData()).(interface{ Validate() error }); ok {
		if err := v.Validate(); err != nil {
			return K8SPolicyValidationError{
				field:  "ReporterData",
				reason: "embedded message failed validation",
				cause:  err,
			}
		}
	}

	if m.GetResourceData() == nil {
		err := K8SPolicyValidationError{
			field:  "ResourceData",
			reason: "value is required",
		}
		if !all {
			return err
		}
		errors = append(errors, err)
	}

	if all {
		switch v := interface{}(m.GetResourceData()).(type) {
		case interface{ ValidateAll() error }:
			if err := v.ValidateAll(); err != nil {
				errors = append(errors, K8SPolicyValidationError{
					field:  "ResourceData",
					reason: "embedded message failed validation",
					cause:  err,
				})
			}
		case interface{ Validate() error }:
			if err := v.Validate(); err != nil {
				errors = append(errors, K8SPolicyValidationError{
					field:  "ResourceData",
					reason: "embedded message failed validation",
					cause:  err,
				})
			}
		}
	} else if v, ok := interface{}(m.GetResourceData()).(interface{ Validate() error }); ok {
		if err := v.Validate(); err != nil {
			return K8SPolicyValidationError{
				field:  "ResourceData",
				reason: "embedded message failed validation",
				cause:  err,
			}
		}
	}

	if len(errors) > 0 {
		return K8SPolicyMultiError(errors)
	}

	return nil
}

// K8SPolicyMultiError is an error wrapping multiple validation errors returned
// by K8SPolicy.ValidateAll() if the designated constraints aren't met.
type K8SPolicyMultiError []error

// Error returns a concatenation of all the error messages it wraps.
func (m K8SPolicyMultiError) Error() string {
	var msgs []string
	for _, err := range m {
		msgs = append(msgs, err.Error())
	}
	return strings.Join(msgs, "; ")
}

// AllErrors returns a list of validation violation errors.
func (m K8SPolicyMultiError) AllErrors() []error { return m }

// K8SPolicyValidationError is the validation error returned by
// K8SPolicy.Validate if the designated constraints aren't met.
type K8SPolicyValidationError struct {
	field  string
	reason string
	cause  error
	key    bool
}

// Field function returns field value.
func (e K8SPolicyValidationError) Field() string { return e.field }

// Reason function returns reason value.
func (e K8SPolicyValidationError) Reason() string { return e.reason }

// Cause function returns cause value.
func (e K8SPolicyValidationError) Cause() error { return e.cause }

// Key function returns key value.
func (e K8SPolicyValidationError) Key() bool { return e.key }

// ErrorName returns error name.
func (e K8SPolicyValidationError) ErrorName() string { return "K8SPolicyValidationError" }

// Error satisfies the builtin error interface
func (e K8SPolicyValidationError) Error() string {
	cause := ""
	if e.cause != nil {
		cause = fmt.Sprintf(" | caused by: %v", e.cause)
	}

	key := ""
	if e.key {
		key = "key for "
	}

	return fmt.Sprintf(
		"invalid %sK8SPolicy.%s: %s%s",
		key,
		e.field,
		e.reason,
		cause)
}

var _ error = K8SPolicyValidationError{}

var _ interface {
	Field() string
	Reason() string
	Key() bool
	Cause() error
	ErrorName() string
} = K8SPolicyValidationError{}