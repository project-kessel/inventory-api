package model

import (
	"fmt"
	"strings"

	"github.com/xeipuuv/gojsonschema"
)

type ValidationSchema interface {
	Validate(data interface{}) (bool, error)
}
type ValidationSchemaFromString func(string) ValidationSchema

type jsonSchemaValidator struct {
	jsonSchema string
}

func NewJsonSchemaValidatorFromString(jsonSchema string) ValidationSchema {
	return jsonSchemaValidator{
		jsonSchema: jsonSchema,
	}
}
func (jschema jsonSchemaValidator) Validate(data interface{}) (bool, error) {
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
