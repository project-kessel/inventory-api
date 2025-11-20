package validation

type Schema interface {
	Validate(data interface{}) (bool, error)
}

type SchemaFromString func(string) Schema
