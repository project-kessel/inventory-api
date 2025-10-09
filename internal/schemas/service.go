package schemas

import (
	"context"
	"fmt"
	"github.com/project-kessel/inventory-api/internal/middleware"
)

const CommonType = "common"
const ReporterType = "reporter"

type SchemaServiceImpl struct {
	repository SchemaRepository
}

func newSchemaService(repository SchemaRepository) *SchemaServiceImpl {
	return &SchemaServiceImpl{repository: repository}
}

func (s *SchemaServiceImpl) validateWithResourceType(ctx context.Context, resourceType string, representation map[string]interface{}, dataType string) error {
	schema, err := s.repository.Get(ctx, SchemaType{
		ResourceType: resourceType,
		Type:         dataType,
	})

	if err != nil {
		return err
	}

	if schema == "" && len(representation) > 0 {
		return fmt.Errorf("%s representation provided but no %s schema present", dataType, dataType)
	}

	err = middleware.ValidateJSONSchema(schema, representation)
	if err != nil {
		return err
	}

	return nil
}

func (s *SchemaServiceImpl) ShallowValidate(ctx context.Context, resourceType string, commonRepresentation map[string]interface{}, reporterRepresentation map[string]interface{}) error {

	err := s.validateWithResourceType(ctx, resourceType, commonRepresentation, CommonType)
	if err != nil {
		return err
	}

	err = s.validateWithResourceType(ctx, resourceType, reporterRepresentation, ReporterType)
	if err != nil {
		return err
	}

	return nil
}
