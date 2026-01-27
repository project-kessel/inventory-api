package middleware

import (
	"context"
	"fmt"
	"strings"
)

type contextKey struct {
	name string
}

func GetFromContext[V any, I any](i I) func(context.Context) (*V, error) {
	typ := fmt.Sprintf("%T", new(V))

	return (func(ctx context.Context) (*V, error) {
		obj := ctx.Value(i)
		if obj == nil {
			return nil, fmt.Errorf("expected %s", typ)
		}
		req, ok := obj.(*V)
		if !ok {
			return nil, fmt.Errorf("object stored in request context couldn't convert to %s", typ)
		}
		return req, nil

	})
}

// RemoveNulls recursively creates a new map with keys removed where the value is null.
// This function is safe for concurrent use as it does not modify the input map.
func RemoveNulls(m map[string]interface{}) map[string]interface{} {
	// Fast path: check if any nulls exist at any depth
	if !hasNullsRecursive(m) {
		return m
	}

	result := make(map[string]interface{}, len(m))
	for key, val := range m {
		if val == nil {
			continue
		}

		switch v := val.(type) {
		case string:
			if strings.EqualFold(v, "null") {
				continue
			}
			result[key] = v

		case map[string]interface{}:
			cleaned := RemoveNulls(v)
			if len(cleaned) > 0 {
				result[key] = cleaned
			}

		default:
			result[key] = val
		}
	}
	return result
}

// hasNullsRecursive recursively checks for nil values or "null" strings in a map
func hasNullsRecursive(m map[string]interface{}) bool {
	for _, val := range m {
		if val == nil {
			return true
		}
		switch v := val.(type) {
		case string:
			if strings.EqualFold(v, "null") {
				return true
			}
		case map[string]interface{}:
			if hasNullsRecursive(v) {
				return true
			}
		}
	}
	return false
}
