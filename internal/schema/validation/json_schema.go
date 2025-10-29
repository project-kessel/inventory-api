package validation

import (
	"fmt"
	"strings"

	"github.com/project-kessel/inventory-api/internal/schema/api"
	"github.com/xeipuuv/gojsonschema"
)

type jsonSchemaValidator struct {
	jsonSchema string
}

func NewJsonSchemaValidatorFromString(jsonSchema string) api.ValidationSchema {
	return jsonSchemaValidator{
		jsonSchema: jsonSchema,
	}
}

func (schema jsonSchemaValidator) Validate(data interface{}) (bool, error) {
	schemaLoader := gojsonschema.NewStringLoader(schema.jsonSchema)
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
