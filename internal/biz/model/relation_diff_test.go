package model_test

import (
	"testing"

	"github.com/project-kessel/inventory-api/internal/biz/model"
	"github.com/stretchr/testify/assert"
)

func TestDiffRelationValues(t *testing.T) {
	key := newTestReporterResourceKey(t)

	tests := []struct {
		name           string
		currentValues  []string
		previousValues []string
		expectCreates  int
		expectDeletes  int
	}{
		{
			name:           "initial creation produces creates only",
			currentValues:  []string{"ws-1", "ws-2"},
			previousValues: nil,
			expectCreates:  2,
			expectDeletes:  0,
		},
		{
			name:           "no change produces no tuples",
			currentValues:  []string{"ws-1", "ws-2"},
			previousValues: []string{"ws-1", "ws-2"},
			expectCreates:  0,
			expectDeletes:  0,
		},
		{
			name:           "addition produces create",
			currentValues:  []string{"ws-1", "ws-2", "ws-3"},
			previousValues: []string{"ws-1", "ws-2"},
			expectCreates:  1,
			expectDeletes:  0,
		},
		{
			name:           "removal produces delete",
			currentValues:  []string{"ws-1"},
			previousValues: []string{"ws-1", "ws-2"},
			expectCreates:  0,
			expectDeletes:  1,
		},
		{
			name:           "full replacement produces creates and deletes",
			currentValues:  []string{"ws-3", "ws-4"},
			previousValues: []string{"ws-1", "ws-2"},
			expectCreates:  2,
			expectDeletes:  2,
		},
		{
			name:           "both empty produces no tuples",
			currentValues:  nil,
			previousValues: nil,
			expectCreates:  0,
			expectDeletes:  0,
		},
		{
			name:           "deletion of all values produces deletes only",
			currentValues:  nil,
			previousValues: []string{"ws-1", "ws-2"},
			expectCreates:  0,
			expectDeletes:  2,
		},
		{
			name:           "single valued relation change",
			currentValues:  []string{"parent-new"},
			previousValues: []string{"parent-old"},
			expectCreates:  1,
			expectDeletes:  1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			creates, deletes := model.DiffRelationValues(
				key, "allowed_workspaces", "rbac", "workspace",
				tt.currentValues, tt.previousValues,
			)
			assert.Len(t, creates, tt.expectCreates)
			assert.Len(t, deletes, tt.expectDeletes)
		})
	}
}

func TestDiffRelationValues_TupleContent(t *testing.T) {
	key := newTestReporterResourceKey(t)

	creates, deletes := model.DiffRelationValues(
		key, "billing_account", "features", "billing_account",
		[]string{"ba-new"}, []string{"ba-old"},
	)

	assert.Len(t, creates, 1)
	assert.Equal(t, "billing_account", creates[0].Relation().Serialize())
	assert.Equal(t, "ba-new", creates[0].Subject().Resource().ResourceId().Serialize())
	assert.Equal(t, "features", creates[0].Subject().Resource().Reporter().ReporterType().Serialize())

	assert.Len(t, deletes, 1)
	assert.Equal(t, "billing_account", deletes[0].Relation().Serialize())
	assert.Equal(t, "ba-old", deletes[0].Subject().Resource().ResourceId().Serialize())
}
