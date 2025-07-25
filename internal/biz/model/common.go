package model

import (
	"fmt"
	"net/url"
)

// ValidationError represents a domain validation error
// Deprecated: Use DomainValidationError instead for better error handling with sentinel errors
type ValidationError struct {
	Field   string
	Message string
}

func (e ValidationError) Error() string {
	return fmt.Sprintf("%s: %s", e.Field, e.Message)
}

type JsonObject map[string]interface{}

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
