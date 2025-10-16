package middleware

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
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

func MarshalProtoToJSON(msg proto.Message) ([]byte, error) {
	data, err := protojson.Marshal(msg)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal message: %w", err)
	}
	return data, nil
}

func UnmarshalJSONToMap(data []byte) (map[string]interface{}, error) {
	var resourceMap map[string]interface{}
	if err := json.Unmarshal(data, &resourceMap); err != nil {
		return nil, fmt.Errorf("failed to unmarshal JSON: %w", err)
	}
	return resourceMap, nil
}

// ExtractOption configures extraction behavior
type ExtractOption func(*extractConfig)

type extractConfig struct {
	validateFieldExists bool
}

// ValidateFieldExists makes the extraction fail if the field doesn't exist
func ValidateFieldExists() ExtractOption {
	return func(c *extractConfig) {
		c.validateFieldExists = true
	}
}

// Extracts a Map Field from another map
func ExtractMapField(data map[string]interface{}, key string, opts ...ExtractOption) (map[string]interface{}, error) {
	config := &extractConfig{validateFieldExists: false}
	for _, opt := range opts {
		opt(config)
	}

	value, exists := data[key]
	if !exists {
		if config.validateFieldExists {
			return nil, fmt.Errorf("missing '%s' field in payload", key)
		}
		return nil, nil // Return nil without error when field doesn't exist and not required
	}

	mapValue, ok := value.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("'%s' is not a valid object", key)
	}

	return mapValue, nil
}

// Extracts a String Field from a map
func ExtractStringField(data map[string]interface{}, key string, opts ...ExtractOption) (string, error) {
	config := &extractConfig{validateFieldExists: false}
	for _, opt := range opts {
		opt(config)
	}

	value, exists := data[key]
	if !exists {
		if config.validateFieldExists {
			return "", fmt.Errorf("missing '%s' field in payload", key)
		}
		return "", nil // Return empty string without error when field doesn't exist and not required
	}

	strValue, ok := value.(string)
	if !ok || strValue == "" {
		return "", fmt.Errorf("'%s' is not a valid string (got %T instead)", key, value)
	}

	return strValue, nil
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
