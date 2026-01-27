package data

import (
	"fmt"
	"strings"

	"github.com/project-kessel/inventory-api/internal/biz/model"
	"github.com/xeipuuv/gojsonschema"
)

// JsonSchemaWithWorkspaces is a schema implementation that validates data using JSON Schema
// and calculates workspace-based relation tuples for replication.
type JsonSchemaWithWorkspaces struct {
	jsonSchema string
}

// NewJsonSchemaWithWorkspacesFromString creates a new JsonSchemaWithWorkspaces from a JSON schema string.
func NewJsonSchemaWithWorkspacesFromString(jsonSchema string) model.Schema {
	return JsonSchemaWithWorkspaces{
		jsonSchema: jsonSchema,
	}
}

// Validate validates the given data against the JSON schema.
func (jschema JsonSchemaWithWorkspaces) Validate(data interface{}) (bool, error) {
	schemaLoader := gojsonschema.NewStringLoader(jschema.jsonSchema)
	dataLoader := gojsonschema.NewGoLoader(data)
	result, err := gojsonschema.Validate(schemaLoader, dataLoader)
	if err != nil {
		return false, fmt.Errorf("validation error: %w", err)
	}
	if !result.Valid() {
		var errMsgs []string
		for _, desc := range result.Errors() {
			errMsgs = append(errMsgs, desc.String())
		}
		return false, fmt.Errorf("validation failed: %s", strings.Join(errMsgs, "; "))
	}
	return true, nil
}

// CalculateTuples computes the relation tuples to replicate based on current and previous
// representations. It extracts workspace_id from the common representation and determines
// what tuples need to be created or deleted based on workspace changes.
func (jschema JsonSchemaWithWorkspaces) CalculateTuples(current, previous *model.Representations, key model.ReporterResourceKey) (model.TuplesToReplicate, error) {
	// Extract workspace IDs from representations
	// current can be nil for DELETE operations (meaning no current/new state)
	currentWorkspaceID := ""
	if current != nil {
		currentWorkspaceID = current.WorkspaceID()
	}
	previousWorkspaceID := ""
	if previous != nil {
		previousWorkspaceID = previous.WorkspaceID()
	}

	// Handle no-op case where workspace hasn't changed
	if previousWorkspaceID != "" && previousWorkspaceID == currentWorkspaceID {
		return model.TuplesToReplicate{}, nil
	}

	// Build tuples to create and delete
	var tuplesToCreate, tuplesToDelete []model.RelationsTuple

	if currentWorkspaceID != "" {
		tuplesToCreate = append(tuplesToCreate, model.NewWorkspaceRelationsTuple(currentWorkspaceID, key))
	}

	if previousWorkspaceID != "" {
		tuplesToDelete = append(tuplesToDelete, model.NewWorkspaceRelationsTuple(previousWorkspaceID, key))
	}

	return model.NewTuplesToReplicate(tuplesToCreate, tuplesToDelete)
}
