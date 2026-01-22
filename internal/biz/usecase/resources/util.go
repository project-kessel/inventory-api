package resources

import (
	"strings"
)

// removeNulls recursively creates a new map with keys removed where the value is null.
// This function is safe for concurrent use as it does not modify the input map.
func removeNulls(m map[string]interface{}) map[string]interface{} {
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
			cleaned := removeNulls(v)
			if len(cleaned) > 0 {
				result[key] = cleaned
			}

		default:
			result[key] = val
		}
	}
	return result
}

// Recursively checks for nil values or "null" strings in a map
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
