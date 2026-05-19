package data

import (
	"fmt"
	"strings"

	"github.com/project-kessel/inventory-api/internal/biz/model"
	"github.com/xeipuuv/gojsonschema"
)

// JsonSchemaWithWorkspaces is a Schema implementation that validates data using JSON Schema
// and calculates workspace-based tuples for authorization.
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

// CalculateTuples delegates to DefaultSchema for workspace-based tuple calculation.
func (jschema JsonSchemaWithWorkspaces) CalculateTuples(currentRepresentation, previousRepresentation *model.Representations, key model.ReporterResourceKey) (model.TuplesToReplicate, error) {
	return model.NewDefaultSchema().CalculateTuples(currentRepresentation, previousRepresentation, key)
}
