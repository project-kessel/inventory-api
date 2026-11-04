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

func TestRepresentations_ReporterBoolField(t *testing.T) {
	reporter := func(data map[string]interface{}) *model.Representations {
		ver := model.NewVersion(0)
		rep, err := model.NewRepresentations(nil, nil, model.Representation(data), &ver)
		require.NoError(t, err)
		return rep
	}

	assert.True(t, reporter(map[string]interface{}{"f": true}).ReporterBoolField("f"))
	assert.False(t, reporter(map[string]interface{}{"f": false}).ReporterBoolField("f"))
	assert.False(t, reporter(map[string]interface{}{}).ReporterBoolField("f"), "missing")
	assert.False(t, reporter(map[string]interface{}{"f": "features/service:*"}).ReporterBoolField("f"), "non-boolean")

	ver := model.NewVersion(0)
	commonOnly, err := model.NewRepresentations(model.Representation(map[string]interface{}{"f": true}), &ver, nil, nil)
	require.NoError(t, err)
	assert.False(t, commonOnly.ReporterBoolField("f"), "common representation is not read")

	var nilRep *model.Representations
	assert.False(t, nilRep.ReporterBoolField("f"))
}

func TestNewRepresentations_Validation(t *testing.T) {
	ver := model.NewVersion(1)

	t.Run("accepts nil data with version (tombstone case)", func(t *testing.T) {
		// Tombstones have nil/empty data but a version
		rep, err := model.NewRepresentations(nil, nil, nil, &ver)
		require.NoError(t, err)
		require.NotNil(t, rep)
		assert.Nil(t, rep.ReporterData())
		assert.Equal(t, &ver, rep.ReporterVersion())
	})

	t.Run("accepts empty data with version (tombstone case)", func(t *testing.T) {
		emptyData := model.Representation(map[string]interface{}{})
		rep, err := model.NewRepresentations(nil, nil, emptyData, &ver)
		require.NoError(t, err)
		require.NotNil(t, rep)
		assert.NotNil(t, rep.ReporterData())
		assert.Equal(t, 0, len(rep.ReporterData()))
		assert.Equal(t, &ver, rep.ReporterVersion())
	})

	t.Run("rejects both versions nil", func(t *testing.T) {
		data := model.Representation(map[string]interface{}{"key": "value"})
		rep, err := model.NewRepresentations(data, nil, nil, nil)
		require.Error(t, err)
		assert.Nil(t, rep)
		assert.Contains(t, err.Error(), "at least one version must be present")
	})

	t.Run("rejects non-empty data without version", func(t *testing.T) {
		t.Run("common data without common version", func(t *testing.T) {
			data := model.Representation(map[string]interface{}{"key": "value"})
			reporterVer := model.NewVersion(1)

			rep, err := model.NewRepresentations(data, nil, nil, &reporterVer)
			require.Error(t, err)
			assert.Nil(t, rep)
			assert.Contains(t, err.Error(), "common data requires common version")
		})

		t.Run("reporter data without reporter version", func(t *testing.T) {
			data := model.Representation(map[string]interface{}{"key": "value"})
			commonVer := model.NewVersion(1)

			rep, err := model.NewRepresentations(nil, &commonVer, data, nil)
			require.Error(t, err)
			assert.Nil(t, rep)
			assert.Contains(t, err.Error(), "reporter data requires reporter version")
		})
	})

	t.Run("accepts both common and reporter", func(t *testing.T) {
		commonData := model.Representation(map[string]interface{}{"common": "data"})
		reporterData := model.Representation(map[string]interface{}{"reporter": "data"})
		commonVer := model.NewVersion(1)
		reporterVer := model.NewVersion(2)

		rep, err := model.NewRepresentations(commonData, &commonVer, reporterData, &reporterVer)
		require.NoError(t, err)
		require.NotNil(t, rep)
		assert.True(t, rep.HasCommon())
		assert.True(t, rep.HasReporter())
	})
}
