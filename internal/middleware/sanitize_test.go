package middleware

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRemoveNulls(t *testing.T) {
	tests := []struct {
		name     string
		input    map[string]interface{}
		expected map[string]interface{}
	}{
		{
			name:     "nil values removed",
			input:    map[string]interface{}{"a": "1", "b": nil},
			expected: map[string]interface{}{"a": "1"},
		},
		{
			name:     "string null removed",
			input:    map[string]interface{}{"a": "1", "b": "null"},
			expected: map[string]interface{}{"a": "1"},
		},
		{
			name:     "string NULL removed (case insensitive)",
			input:    map[string]interface{}{"a": "1", "b": "NULL"},
			expected: map[string]interface{}{"a": "1"},
		},
		{
			name:     "nested nil removed",
			input:    map[string]interface{}{"a": map[string]interface{}{"b": nil, "c": "1"}},
			expected: map[string]interface{}{"a": map[string]interface{}{"c": "1"}},
		},
		{
			name:     "no nulls returns original",
			input:    map[string]interface{}{"a": "1", "b": "2"},
			expected: map[string]interface{}{"a": "1", "b": "2"},
		},
		{
			name:     "all nulls returns empty map",
			input:    map[string]interface{}{"a": nil, "b": "null"},
			expected: map[string]interface{}{},
		},
		{
			name:     "empty nested map removed",
			input:    map[string]interface{}{"a": map[string]interface{}{"b": nil}},
			expected: map[string]interface{}{},
		},
		{
			name:     "non-string non-map values preserved",
			input:    map[string]interface{}{"a": 123, "b": true, "c": nil},
			expected: map[string]interface{}{"a": 123, "b": true},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := RemoveNulls(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}
