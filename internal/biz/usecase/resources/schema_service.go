package resources

import (
	"github.com/go-kratos/kratos/v2/log"
	"github.com/project-kessel/inventory-api/internal/biz/model"
)

type SchemaUsecase struct {
	Log *log.Helper
}

func NewSchemaUsecase(logger *log.Helper) *SchemaUsecase {
	return &SchemaUsecase{
		Log: logger,
	}
}

func (sc *SchemaUsecase) CalculateTuples(currentRepresentation, previousRepresentation *model.Representations, key model.ReporterResourceKey) (model.TuplesToReplicate, error) {
	// Extract workspace IDs from representations
	// currentRepresentation can be nil for DELETE operations (meaning no current/new state)
	currentWorkspaceID := ""
	if currentRepresentation != nil {
		currentWorkspaceID = currentRepresentation.WorkspaceID()
	}
	previousWorkspaceID := ""
	if previousRepresentation != nil {
		previousWorkspaceID = previousRepresentation.WorkspaceID()
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
