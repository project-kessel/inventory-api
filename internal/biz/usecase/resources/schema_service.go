package resources

import (
	"github.com/go-kratos/kratos/v2/log"
	"github.com/project-kessel/inventory-api/internal/biz/model"
	"github.com/project-kessel/inventory-api/internal/data"
)

type SchemaUsecase struct {
	Log *log.Helper
}

func NewSchemaUsecase(logger *log.Helper) *SchemaUsecase {
	return &SchemaUsecase{
		Log: logger,
	}
}

func (sc *SchemaUsecase) CalculateTuples(representations []data.RepresentationsByVersion, key model.ReporterResourceKey) (model.TuplesToReplicate, error) {
	if len(representations) == 0 {
		return model.TuplesToReplicate{}, nil
	}

	// Identify current (max version) and previous (next lower version)
	var maxVer uint
	for _, r := range representations {
		if r.Version > maxVer {
			maxVer = r.Version
		}
	}
	var (
		currentWorkspaceID  string
		previousWorkspaceID string
	)
	for _, r := range representations {
		if r.Version == maxVer && currentWorkspaceID == "" {
			currentWorkspaceID = data.ExtractWorkspaceID(r)
		}
	}
	// find the largest version lower than maxVer
	var prevVer uint
	for _, r := range representations {
		if r.Version < maxVer && r.Version > prevVer {
			prevVer = r.Version
			previousWorkspaceID = data.ExtractWorkspaceID(r)
		}
	}

	return sc.BuildTuplesToReplicate(currentWorkspaceID, previousWorkspaceID, key)
}

func (sc *SchemaUsecase) BuildTuplesToReplicate(currentWorkspaceID, previousWorkspaceID string, key model.ReporterResourceKey) (model.TuplesToReplicate, error) {
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

	return model.NewTuplesToReplicate(tuplesToCreate, tuplesToDelete)
}
