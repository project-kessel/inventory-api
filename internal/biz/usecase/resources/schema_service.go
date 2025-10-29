package resources

import (
	"fmt"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/project-kessel/inventory-api/internal/biz"
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

// CalculateTuples : For now we are determining ReportTupleEvent vs DeleteTupleEvent based on Operation Type, but the eventual goal is for input to be ReportResourceEvent and DeleteResourceEvent
func (sc *SchemaUsecase) CalculateTuples(tupleEvent model.TupleEvent, operationType biz.EventOperationType) (model.TuplesToReplicate, error) {

	sc.Log.Infof("Calculating Tuples for operationType and event: %d, key: %+v", operationType, tupleEvent)

	switch operationType.OperationType() {
	case biz.OperationTypeDeleted:
		return sc.processDeleteTupleEvent(tupleEvent)
	default:
		return sc.processReportTupleEvent(tupleEvent, operationType)
	}
}

func (sc *SchemaUsecase) processReportTupleEvent(tupleEvent model.TupleEvent, operationType biz.EventOperationType) (model.TuplesToReplicate, error) {
	key := tupleEvent.ReporterResourceKey()

	if tupleEvent.CommonVersion() == nil {
		return model.TuplesToReplicate{}, nil
	}

	version := tupleEvent.CommonVersion().Uint()
	currentVersion := &version

	representations, err := sc.resourceRepository.FindCurrentAndPreviousVersionedRepresentations(
		nil, key, currentVersion, operationType,
	)
	if err != nil {
		return model.TuplesToReplicate{}, fmt.Errorf("failed to find representations: %w", err)
	}

	currentWorkspaceID, previousWorkspaceID := data.GetCurrentAndPreviousWorkspaceID(representations, version)
	// Detailed log for tuple calculation across generation boundaries
	{
		localId, resType, repType, repInst := key.LocalResourceId().Serialize(), key.ResourceType().Serialize(), key.ReporterType().Serialize(), key.ReporterInstanceId().Serialize()
		sc.Log.Infof("TuplesToReplicate[report]: op=%s localResourceId=%s resourceType=%s reporterType=%s reporterInstanceId=%s currentCommonVersion=%d currentWorkspace=%q previousWorkspace=%q", operationType.OperationType(), localId, resType, repType, repInst, version, currentWorkspaceID, previousWorkspaceID)
	}
	return sc.buildTuplesToReplicate(currentWorkspaceID, previousWorkspaceID, key)
}

func (sc *SchemaUsecase) processDeleteTupleEvent(tupleEvent model.TupleEvent) (model.TuplesToReplicate, error) {
	key := tupleEvent.ReporterResourceKey()

	representation, err := sc.resourceRepository.FindLatestRepresentations(nil, key)
	if err != nil {
		return model.TuplesToReplicate{}, fmt.Errorf("failed to find representations: %w", err)
	}

	currentWorkspaceID := data.ExtractWorkspaceID(representation)
	{
		localId, resType, repType, repInst := key.LocalResourceId().Serialize(), key.ResourceType().Serialize(), key.ReporterType().Serialize(), key.ReporterInstanceId().Serialize()
		sc.Log.Infof("TuplesToReplicate[delete]: localResourceId=%s resourceType=%s reporterType=%s reporterInstanceId=%s latestCommonVersion=%d workspaceToDelete=%q", localId, resType, repType, repInst, representation.Version, currentWorkspaceID)
	}
	return sc.buildTuplesToReplicate("", currentWorkspaceID, key)
}

func (sc *SchemaUsecase) buildTuplesToReplicate(currentWorkspaceID, previousWorkspaceID string, key model.ReporterResourceKey) (model.TuplesToReplicate, error) {
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

	tuples, err := model.NewTuplesToReplicate(tuplesToCreate, tuplesToDelete)
	if err != nil {
		return model.TuplesToReplicate{}, err
	}
	{
		localId, resType, repType, repInst := key.LocalResourceId().Serialize(), key.ResourceType().Serialize(), key.ReporterType().Serialize(), key.ReporterInstanceId().Serialize()
		sc.Log.Infof("TuplesToReplicate[result]: localResourceId=%s resourceType=%s reporterType=%s reporterInstanceId=%s create=%d delete=%d", localId, resType, repType, repInst, len(tuplesToCreate), len(tuplesToDelete))
	}
	return tuples, nil
}
