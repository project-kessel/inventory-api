package schema

import (
	"context"
	"fmt"

	"github.com/go-kratos/kratos/v2/errors"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/project-kessel/inventory-api/internal/schema/api"
)

type SchemaService struct {
	repository api.SchemaRepository
}

func NewSchemaService(repository api.SchemaRepository) *SchemaService {
	return &SchemaService{repository: repository}
}

// IsReporterForResource validates the resourceType and reporterType combination is valid. i.e. that there is a reporter that reports said resource.
func (s *SchemaService) IsReporterForResource(ctx context.Context, resourceType string, reporterType string) (bool, error) {
	if _, err := s.repository.GetReporterSchema(ctx, resourceType, reporterType); err != nil {
		if errors.Is(err, api.ResourceSchemaNotFound) || errors.Is(err, api.ReporterSchemaNotfound) {
			return false, nil
		}

		return false, err
	}

	return true, nil
}

// CommonShallowValidate validates the common representation for a given resourceType.
func (s *SchemaService) CommonShallowValidate(ctx context.Context, resourceType string, commonRepresentation map[string]interface{}) error {
	resource, err := s.repository.GetResourceSchema(ctx, resourceType)
	if err != nil {
		return fmt.Errorf("failed to load common representation schema for '%s': %w", resourceType, err)
	}

	if resource.CommonRepresentationSchema == "" {
		return fmt.Errorf("no schema found for '%s'", resourceType)
	}

	hasCommonRepresentationData := len(commonRepresentation) > 0
	if !hasCommonRepresentationData {
		commonRepresentation = map[string]interface{}{}
	}

	err = validateJSONSchema(resource.CommonRepresentationSchema, commonRepresentation)
	if err != nil {
		if hasCommonRepresentationData {
			return err
		}
		return fmt.Errorf("missing 'common' field in payload - schema for '%s' has required fields: %w", resourceType, err)
	}

	return nil
}

// ReporterShallowValidate validates the specific reporter representation for a given resourceType/reporterType.
func (s *SchemaService) ReporterShallowValidate(ctx context.Context, resourceType string, reporterType string, reporterRepresentation map[string]interface{}) error {
	reporter, err := s.repository.GetReporterSchema(ctx, resourceType, reporterType)
	if err != nil {
		return err
	}

	// Case 1: No schema found for resourceType:reporterType
	if reporter.ReporterRepresentationSchema == "" {
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

	err = validateJSONSchema(reporter.ReporterRepresentationSchema, reporterRepresentation)
	if err != nil {
		if hasReporterRepresentationData {
			return err
		}

		// If schema has validation errors but reporterRepresentation is nil/empty, that's an error
		return fmt.Errorf("missing 'reporter' field in payload - schema for '%s:%s' has required fields: %w", resourceType, reporterType, err)
	}

	return nil
}
