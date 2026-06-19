package model_test

import (
	"testing"

	"github.com/project-kessel/inventory-api/internal/biz/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRepresentations_StringField(t *testing.T) {
	tests := []struct {
		name     string
		data     map[string]interface{}
		field    string
		expected string
	}{
		{
			name:     "returns string value",
			data:     map[string]interface{}{"workspace_id": "ws-1"},
			field:    "workspace_id",
			expected: "ws-1",
		},
		{
			name:     "returns empty for missing field",
			data:     map[string]interface{}{"other": "value"},
			field:    "workspace_id",
			expected: "",
		},
		{
			name:     "returns empty for wrong type",
			data:     map[string]interface{}{"workspace_id": 123},
			field:    "workspace_id",
			expected: "",
		},
		{
			name:     "returns empty for nil value",
			data:     map[string]interface{}{"workspace_id": nil},
			field:    "workspace_id",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ver := model.NewVersion(0)
			rep, err := model.NewRepresentations(
				model.Representation(tt.data), &ver, nil, nil,
			)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, rep.StringField(tt.field))
		})
	}

	t.Run("nil receiver returns empty", func(t *testing.T) {
		var rep *model.Representations
		assert.Equal(t, "", rep.StringField("anything"))
	})
}

func TestRepresentations_StringSliceField(t *testing.T) {
	tests := []struct {
		name     string
		data     map[string]interface{}
		field    string
		expected []string
	}{
		{
			name:     "returns string slice",
			data:     map[string]interface{}{"workspaces": []interface{}{"ws-1", "ws-2"}},
			field:    "workspaces",
			expected: []string{"ws-1", "ws-2"},
		},
		{
			name:     "returns nil for missing field",
			data:     map[string]interface{}{"other": "value"},
			field:    "workspaces",
			expected: nil,
		},
		{
			name:     "returns nil for non-array",
			data:     map[string]interface{}{"workspaces": "not-an-array"},
			field:    "workspaces",
			expected: nil,
		},
		{
			name:     "returns nil for nil value",
			data:     map[string]interface{}{"workspaces": nil},
			field:    "workspaces",
			expected: nil,
		},
		{
			name:     "skips non-string elements",
			data:     map[string]interface{}{"workspaces": []interface{}{"ws-1", 123, "ws-2"}},
			field:    "workspaces",
			expected: []string{"ws-1", "ws-2"},
		},
		{
			name:     "returns nil for empty array",
			data:     map[string]interface{}{"workspaces": []interface{}{}},
			field:    "workspaces",
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ver := model.NewVersion(0)
			rep, err := model.NewRepresentations(
				model.Representation(tt.data), &ver, nil, nil,
			)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, rep.StringSliceField(tt.field))
		})
	}

	t.Run("nil receiver returns nil", func(t *testing.T) {
		var rep *model.Representations
		assert.Nil(t, rep.StringSliceField("anything"))
	})
}

func TestRepresentations_WorkspaceID_UsesStringField(t *testing.T) {
	ver := model.NewVersion(0)
	rep, err := model.NewRepresentations(
		model.Representation(map[string]interface{}{"workspace_id": "ws-1"}),
		&ver, nil, nil,
	)
	require.NoError(t, err)
	assert.Equal(t, "ws-1", rep.WorkspaceID())
	assert.Equal(t, rep.StringField("workspace_id"), rep.WorkspaceID())
}
