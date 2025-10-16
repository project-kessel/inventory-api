package schemas

import (
	"context"
	"fmt"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/project-kessel/inventory-api/internal/schemas/api"
)

type SchemaServiceImpl struct {
	repository api.SchemaRepository
}

func NewSchemaService(repository api.SchemaRepository) api.SchemaService {
	return &SchemaServiceImpl{repository: repository}
}

func (s *SchemaServiceImpl) ValidateReporterForResource(ctx context.Context, resourceType string, reporterType string) error {
	if _, err := s.repository.GetResourceReporter(ctx, resourceType, reporterType); err != nil {
		return err
	}

	return nil
}

func (s *SchemaServiceImpl) CommonShallowValidate(ctx context.Context, resourceType string, commonRepresentation map[string]interface{}) error {
	resource, err := s.repository.GetResource(ctx, resourceType)
	if err != nil {
		return fmt.Errorf("failed to load common representation schema for '%s': %w", resourceType, err)
	}

	if resource.CommonSchema == "" {
		return fmt.Errorf("no schema found for '%s'", resourceType)
	}

	hasCommonRepresentationData := len(commonRepresentation) > 0
	if !hasCommonRepresentationData {
		commonRepresentation = map[string]interface{}{}
	}

	err = validateJSONSchema(resource.CommonSchema, commonRepresentation)
	if err != nil {
		if hasCommonRepresentationData {
			return err
		}
		return fmt.Errorf("missing 'common' field in payload - schema for '%s' has required fields: %w", resourceType, err)
	}

	return nil
}

func (s *SchemaServiceImpl) ReporterShallowValidate(ctx context.Context, resourceType string, reporterType string, reporterRepresentation map[string]interface{}) error {
	reporter, err := s.repository.GetResourceReporter(ctx, resourceType, reporterType)
	if err != nil {
		return err
	}

	// Case 1: No schema found for resourceType:reporterType
	if reporter.ReporterSchema == "" {
		if len(reporterRepresentation) > 0 {
			return fmt.Errorf("no schema found for '%s:%s', but reporter representation was provided. Submission is not allowed", resourceType, reporterType)
		}
		log.Debugf("no schema found for %s:%s, treating as abstract reporter representation", resourceType, reporterType)
		return nil
	}

	hasReporterRepresentationData := len(reporterRepresentation) > 0
	if !hasReporterRepresentationData {
		reporterRepresentation = map[string]interface{}{}
	}

	err = validateJSONSchema(reporter.ReporterSchema, reporterRepresentation)
	if err != nil {
		if hasReporterRepresentationData {
			return err
		}

		// If schema has validation errors but reporterRepresentation is nil/empty, that's an error
		return fmt.Errorf("missing 'reporter' field in payload - schema for '%s:%s' has required fields: %w", resourceType, reporterType, err)
	}

	return nil
}
