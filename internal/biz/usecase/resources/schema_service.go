package resources

import (
	"fmt"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/project-kessel/inventory-api/internal/biz/model"
	"github.com/project-kessel/inventory-api/internal/data"
)

type SchemaUsecase struct {
	resourceRepository data.ResourceRepository
	Log                *log.Helper
}

func NewSchemaUsecase(resourceRepository data.ResourceRepository, logger *log.Helper) *SchemaUsecase {
	return &SchemaUsecase{
		resourceRepository: resourceRepository,
		Log:                logger,
	}
}

func (sc *SchemaUsecase) CalculateTuples(tupleEvent model.TupleEvent) (model.TuplesToReplicate, error) {
	currentVersion := tupleEvent.Version().Uint()
	key := tupleEvent.ReporterResourceKey()

	sc.Log.Infof("CalculateTuples called - version: %d, key: %+v", currentVersion, key)

	versionedRepresentations, err := sc.getWorkspaceVersions(key, currentVersion)
	if err != nil {
		return model.TuplesToReplicate{}, err
	}
	return sc.determineTupleOperations(versionedRepresentations, currentVersion, key)
}

func (sc *SchemaUsecase) determineTupleOperations(representationVersion []data.RepresentationsByVersion, currentVersion uint, key model.ReporterResourceKey) (model.TuplesToReplicate, error) {
	currentWorkspaceID, previousWorkspaceID := data.GetCurrentAndPreviousWorkspaceID(representationVersion, currentVersion)

	if previousWorkspaceID != "" && previousWorkspaceID == currentWorkspaceID {
		return model.TuplesToReplicate{}, nil
	}

	var tuplesToCreate, tuplesToDelete []model.RelationsTuple

	if currentWorkspaceID != "" {
		tuplesToCreate = append(tuplesToCreate, model.NewWorkspaceRelationsTuple(currentWorkspaceID, key))
	}

	if previousWorkspaceID != "" {
		tuplesToDelete = append(tuplesToDelete, model.NewWorkspaceRelationsTuple(previousWorkspaceID, key))
	}

	var createPtr, deletePtr *[]model.RelationsTuple
	if len(tuplesToCreate) > 0 {
		createPtr = &tuplesToCreate
	}
	if len(tuplesToDelete) > 0 {
		deletePtr = &tuplesToDelete
	}

	return model.NewTuplesToReplicate(createPtr, deletePtr)
}

func (sc *SchemaUsecase) getWorkspaceVersions(key model.ReporterResourceKey, currentVersion uint) ([]data.RepresentationsByVersion, error) {
	representations, err := sc.resourceRepository.FindVersionedRepresentationsByVersion(
		nil, key, currentVersion,
	)
	if err != nil {
		return []data.RepresentationsByVersion{}, fmt.Errorf("failed to find representations: %w", err)
	}
	return representations, nil
}
