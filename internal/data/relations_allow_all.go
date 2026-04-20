package data

import (
	"context"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/project-kessel/inventory-api/internal/biz/model"
)

type AllowAllRelationsRepository struct {
	Logger *log.Helper
}

var _ model.RelationsRepository = &AllowAllRelationsRepository{}

func NewAllowAllRelationsRepository(logger *log.Helper) *AllowAllRelationsRepository {
	logger.Info("Using relations repository: allow-all")
	return &AllowAllRelationsRepository{
		Logger: logger,
	}
}

func (a *AllowAllRelationsRepository) Health(_ context.Context) (model.HealthResult, error) {
	return model.HealthResult{Status: "OK", Code: 200}, nil
}

func (a *AllowAllRelationsRepository) Check(_ context.Context, _ model.ReporterResourceKey, _ model.Relation,
	_ model.SubjectReference, _ model.Consistency,
) (model.CheckResult, error) {
	return model.CheckResult{Allowed: true}, nil
}

func (a *AllowAllRelationsRepository) CheckForUpdate(_ context.Context, _ model.ReporterResourceKey, _ model.Relation,
	_ model.SubjectReference,
) (model.CheckResult, error) {
	return model.CheckResult{Allowed: true}, nil
}

func (a *AllowAllRelationsRepository) CheckBulk(_ context.Context, items []model.CheckBulkItem, _ model.Consistency,
) (model.CheckBulkResult, error) {
	pairs := make([]model.CheckBulkResultPair, len(items))
	for i, item := range items {
		pairs[i] = model.CheckBulkResultPair{
			Request: item,
			Result:  model.CheckBulkResultItem{Allowed: true},
		}
	}
	return model.CheckBulkResult{Pairs: pairs}, nil
}

func (a *AllowAllRelationsRepository) CheckForUpdateBulk(_ context.Context, items []model.CheckBulkItem,
) (model.CheckBulkResult, error) {
	pairs := make([]model.CheckBulkResultPair, len(items))
	for i, item := range items {
		pairs[i] = model.CheckBulkResultPair{
			Request: item,
			Result:  model.CheckBulkResultItem{Allowed: true},
		}
	}
	return model.CheckBulkResult{Pairs: pairs}, nil
}

func (a *AllowAllRelationsRepository) LookupResources(_ context.Context, _ model.ResourceType, _ model.ReporterType,
	_ model.Relation, _ model.SubjectReference, _ *model.Pagination, _ model.Consistency,
) (model.ResultStream[model.LookupResourcesItem], error) {
	return &emptyLookupResourcesStream{}, nil
}

func (a *AllowAllRelationsRepository) LookupSubjects(_ context.Context, _ model.ReporterResourceKey, _ model.Relation,
	_ model.ResourceType, _ model.ReporterType, _ *model.Relation,
	_ *model.Pagination, _ model.Consistency,
) (model.ResultStream[model.LookupSubjectsItem], error) {
	return &emptyLookupSubjectsStream{}, nil
}

func (a *AllowAllRelationsRepository) CreateTuples(_ context.Context, _ []model.RelationsTuple, _ bool, _ *model.FencingCheck,
) (model.TuplesResult, error) {
	return model.TuplesResult{}, nil
}

func (a *AllowAllRelationsRepository) DeleteTuples(_ context.Context, _ []model.RelationsTuple, _ *model.FencingCheck,
) (model.TuplesResult, error) {
	return model.TuplesResult{}, nil
}

func (a *AllowAllRelationsRepository) DeleteTuplesByFilter(_ context.Context, _ model.TupleFilter, _ *model.FencingCheck,
) (model.TuplesResult, error) {
	return model.TuplesResult{}, nil
}

func (a *AllowAllRelationsRepository) ReadTuples(_ context.Context, _ model.TupleFilter, _ *model.Pagination, _ model.Consistency,
) (model.ResultStream[model.ReadTuplesItem], error) {
	return &emptyReadTuplesStream{}, nil
}

func (a *AllowAllRelationsRepository) AcquireLock(_ context.Context, _ string) (model.AcquireLockResult, error) {
	return model.AcquireLockResult{}, nil
}
