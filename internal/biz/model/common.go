package model

import "fmt"

// ValidationError represents a domain validation error
type ValidationError struct {
	Field   string
	Message string
}

func (e ValidationError) Error() string {
	return fmt.Sprintf("%s: %s", e.Field, e.Message)
}

type JsonObject map[string]interface{}
