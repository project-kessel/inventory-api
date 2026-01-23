package validation

type Schema interface {
	Validate(data interface{}) (bool, error)
	//CalculateTuples(currentRepresentation, previousRepresentation *model.Representations, key model.ReporterResourceKey) (model.TuplesToReplicate, error)
}
type SchemaFromString func(string) Schema
