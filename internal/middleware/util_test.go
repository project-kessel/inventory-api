package middleware_test

import (
	"testing"

	"github.com/project-kessel/inventory-api/internal/middleware"
	"github.com/stretchr/testify/assert"
)

func TestRemoveNulls(t *testing.T) {
	tests := []struct {
		name     string
		input    map[string]interface{}
		expected map[string]interface{}
	}{
		{
			name: "HBI host with all fields",
			input: map[string]interface{}{
				"insights_id":  "b5c36330-79cf-426e-a950-df2e972c3ef4",
				"satellite_id": "some-satellite-id",
				"ansible_host": "my-ansible-host.example.com",
			},
			expected: map[string]interface{}{
				"insights_id":  "b5c36330-79cf-426e-a950-df2e972c3ef4",
				"satellite_id": "some-satellite-id",
				"ansible_host": "my-ansible-host.example.com",
			},
		},
		{
			name: "HBI host with null ansible_host",
			input: map[string]interface{}{
				"insights_id":  "b5c36330-79cf-426e-a950-df2e972c3ef4",
				"satellite_id": "some-satellite-id",
				"ansible_host": nil,
			},
			expected: map[string]interface{}{
				"insights_id":  "b5c36330-79cf-426e-a950-df2e972c3ef4",
				"satellite_id": "some-satellite-id",
			},
		},
		{
			name: "HBI host with multiple nulls",
			input: map[string]interface{}{
				"insights_id":  "b5c36330-79cf-426e-a950-df2e972c3ef4",
				"satellite_id": nil,
				"ansible_host": "null",
			},
			expected: map[string]interface{}{
				"insights_id": "b5c36330-79cf-426e-a950-df2e972c3ef4",
			},
		},
		{
			name: "nested nulls in a generic structure",
			input: map[string]interface{}{
				"metadata": map[string]interface{}{
					"source": "some-source",
					"notes":  nil,
				},
				"data": "some-data",
			},
			expected: map[string]interface{}{
				"metadata": map[string]interface{}{
					"source": "some-source",
				},
				"data": "some-data",
			},
		},
		{
			name:     "empty map",
			input:    map[string]interface{}{},
			expected: map[string]interface{}{},
		},
		{
			name: "nested string 'null' value",
			input: map[string]interface{}{
				"details": map[string]interface{}{
					"comment": "NULL",
					"user":    "alice",
				},
			},
			expected: map[string]interface{}{
				"details": map[string]interface{}{
					"user": "alice",
				},
			},
		},
		{
			name: "nested string 'null' value case insensitive",
			input: map[string]interface{}{
				"details": map[string]interface{}{
					"comment": "null",
					"user":    "alice",
				},
			},
			expected: map[string]interface{}{
				"details": map[string]interface{}{
					"user": "alice",
				},
			},
		},
		{
			name: "nested map becomes empty",
			input: map[string]interface{}{
				"meta": map[string]interface{}{
					"comment": nil,
				},
			},
			expected: map[string]interface{}{},
		},
		{
			name: "deeply nested null values",
			input: map[string]interface{}{
				"a": map[string]interface{}{
					"b": map[string]interface{}{
						"c": nil,
						"d": "valid",
					},
				},
			},
			expected: map[string]interface{}{
				"a": map[string]interface{}{
					"b": map[string]interface{}{
						"d": "valid",
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := middleware.RemoveNulls(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}
