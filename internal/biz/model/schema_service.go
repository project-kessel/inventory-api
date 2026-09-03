package model

import (
	"context"
	"errors"
	"fmt"

	"github.com/go-kratos/kratos/v2/log"
)

// SchemaService is a domain service that orchestrates schema-based operations
// such as validation, tuple calculation, and reporter verification.
type SchemaService struct {
	Log              *log.Helper
	schemaRepository SchemaRepository
}

// NewSchemaService creates a new SchemaService with the given repository and logger.
func NewSchemaService(schemaRepository SchemaRepository, logger *log.Helper) *SchemaService {
	return &SchemaService{
		Log:              logger,
		schemaRepository: schemaRepository,
	}
}

// CalculateTuplesForResource computes the relation tuples to replicate for a given resource.
// It calculates tuples from both reporter-specific schema (if exists) and resource/common schema (if exists),
// then merges the results. This allows reporter schemas to define reporter-specific relations
// while common schemas define common relations (e.g., workspace_id).
func (sc *SchemaService) CalculateTuplesForResource(ctx context.Context, current, previous *Representations, key ReporterResourceKey) (TuplesToReplicate, error) {
	resourceType := key.ResourceType()
	reporterType := key.ReporterType()

	var allCreates, allDeletes []RelationsTuple
	foundReporterSchema := false
	foundResourceSchema := false

	// Get tuples from reporter schema (if exists)
	reporterSchema, err := sc.schemaRepository.GetReporterSchema(ctx, resourceType, reporterType)
	if err != nil {
		// Only continue if schema not found; propagate unexpected errors
		if !errors.Is(err, ErrResourceSchemaNotFound) && !errors.Is(err, ErrReporterSchemaNotFound) {
			return TuplesToReplicate{}, err
		}
	} else if reporterSchema.Schema() != nil {
		foundReporterSchema = true
		reporterTuples, err := reporterSchema.Schema().CalculateTuples(current, previous, key)
		if err != nil {
			return TuplesToReplicate{}, err
		}
		if reporterTuples.HasTuplesToCreate() {
			allCreates = append(allCreates, *reporterTuples.TuplesToCreate()...)
		}
		if reporterTuples.HasTuplesToDelete() {
			allDeletes = append(allDeletes, *reporterTuples.TuplesToDelete()...)
		}
	}

	// Get tuples from resource/common schema (if exists)
	resourceSchema, err := sc.schemaRepository.GetResourceSchema(ctx, resourceType)
	if err != nil {
		// Only continue if schema not found; propagate unexpected errors
		if !errors.Is(err, ErrResourceSchemaNotFound) {
			return TuplesToReplicate{}, err
		}
	} else if resourceSchema.Schema() != nil {
		foundResourceSchema = true
		commonTuples, err := resourceSchema.Schema().CalculateTuples(current, previous, key)
		if err != nil {
			return TuplesToReplicate{}, err
		}
		if commonTuples.HasTuplesToCreate() {
			allCreates = append(allCreates, *commonTuples.TuplesToCreate()...)
		}
		if commonTuples.HasTuplesToDelete() {
			allDeletes = append(allDeletes, *commonTuples.TuplesToDelete()...)
		}
	}

	// If no schemas found at all, use default
	if !foundReporterSchema && !foundResourceSchema {
		return NewDefaultSchema().CalculateTuples(current, previous, key)
	}

	return NewTuplesToReplicate(allCreates, allDeletes)
}

// ValidateReportAgainstSchema validates that a resource report conforms to the configured schemas.
// It checks that the reporter is allowed for the resource type, and validates both
// reporter and common representations against their respective schemas.
func (sc *SchemaService) ValidateReportAgainstSchema(ctx context.Context, resourceType ResourceType, reporterType ReporterType, commonRepresentation, reporterRepresentation *Representation) error {
	if isReporter, err := sc.IsReporterForResource(ctx, resourceType, reporterType); !isReporter {
		if err != nil {
			return err
		}
		return fmt.Errorf("reporter %s does not report resource types: %s", reporterType, resourceType)
	}

	if reporterRepresentation != nil {
		if err := sc.ReporterShallowValidate(ctx, resourceType, reporterType, *reporterRepresentation); err != nil {
			return err
		}
	}

	if commonRepresentation != nil {
		if err := sc.CommonShallowValidate(ctx, resourceType, *commonRepresentation); err != nil {
			return err
		}
	}

	return nil
}

// IsReporterForResource validates the resourceType and reporterType combination is valid.
// Returns true if there is a reporter that reports said resource, false otherwise.
func (sc *SchemaService) IsReporterForResource(ctx context.Context, resourceType ResourceType, reporterType ReporterType) (bool, error) {
	if _, err := sc.schemaRepository.GetReporterSchema(ctx, resourceType, reporterType); err != nil {
		if errors.Is(err, ErrResourceSchemaNotFound) || errors.Is(err, ErrReporterSchemaNotFound) {
			return false, nil
		}

		return false, err
	}

	return true, nil
}

// CommonShallowValidate validates the common representation for a given resourceType.
func (sc *SchemaService) CommonShallowValidate(ctx context.Context, resourceType ResourceType, commonRepresentation Representation) error {
	resource, err := sc.schemaRepository.GetResourceSchema(ctx, resourceType)
	if err != nil {
		return fmt.Errorf("failed to load common representation schema for '%s': %w", resourceType, err)
	}

	if resource.Schema() == nil {
		return fmt.Errorf("no schema found for '%s'", resourceType)
	}

	hasCommonRepresentationData := len(commonRepresentation) > 0
	if !hasCommonRepresentationData {
		commonRepresentation = NewEmptyRepresentation()
	}

	_, err = resource.Schema().Validate(commonRepresentation)
	if err != nil {
		if hasCommonRepresentationData {
			return err
		}
		return fmt.Errorf("missing 'common' field in payload - schema for '%s' has required fields: %w", resourceType, err)
	}

	return nil
}

// ReporterShallowValidate validates the specific reporter representation for a given resourceType/reporterType.
func (sc *SchemaService) ReporterShallowValidate(ctx context.Context, resourceType ResourceType, reporterType ReporterType, reporterRepresentation Representation) error {
	reporter, err := sc.schemaRepository.GetReporterSchema(ctx, resourceType, reporterType)
	if err != nil {
		return err
	}

	// Case 1: No schema found for resourceType:reporterType
	if reporter.Schema() == nil {
		if len(reporterRepresentation) > 0 {
			return fmt.Errorf("no schema found for '%s:%s', but reporter representation was provided. Submission is not allowed", resourceType, reporterType)
		}
		sc.Log.Debugf("no schema found for %s:%s, treating as abstract reporter representation", resourceType, reporterType)
		return nil
	}

	hasReporterRepresentationData := len(reporterRepresentation) > 0
	if !hasReporterRepresentationData {
		reporterRepresentation = NewEmptyRepresentation()
	}

	_, err = reporter.Schema().Validate(reporterRepresentation)
	if err != nil {
		if hasReporterRepresentationData {
			return err
		}

		// If schema has validation errors but reporterRepresentation is nil/empty, that's an error
		return fmt.Errorf("missing 'reporter' field in payload - schema for '%s:%s' has required fields: %w", resourceType, reporterType, err)
	}

	return nil
}
