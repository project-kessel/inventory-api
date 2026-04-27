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
	return model.NewHealthResult("OK", 200), nil
}

func (a *AllowAllRelationsRepository) Check(_ context.Context, _ model.Relationship, _ model.Consistency,
) (model.CheckResult, error) {
	return model.NewCheckResult(true, model.MinimizeLatencyToken), nil
}

func (a *AllowAllRelationsRepository) CheckForUpdate(_ context.Context, _ model.Relationship,
) (model.CheckResult, error) {
	return model.NewCheckResult(true, model.MinimizeLatencyToken), nil
}

func (a *AllowAllRelationsRepository) CheckBulk(_ context.Context, rels []model.Relationship, _ model.Consistency,
) (model.CheckBulkResult, error) {
	pairs := make([]model.CheckBulkResultPair, len(rels))
	for i, rel := range rels {
		pairs[i] = model.NewCheckBulkResultPair(rel, model.NewCheckBulkResultItem(true, nil, 0))
	}
	return model.NewCheckBulkResult(pairs, model.MinimizeLatencyToken), nil
}

func (a *AllowAllRelationsRepository) CheckForUpdateBulk(_ context.Context, rels []model.Relationship,
) (model.CheckBulkResult, error) {
	pairs := make([]model.CheckBulkResultPair, len(rels))
	for i, rel := range rels {
		pairs[i] = model.NewCheckBulkResultPair(rel, model.NewCheckBulkResultItem(true, nil, 0))
	}
	return model.NewCheckBulkResult(pairs, model.MinimizeLatencyToken), nil
}

func (a *AllowAllRelationsRepository) LookupObjects(_ context.Context,
	_ model.RepresentationType,
	_ model.Relation, _ model.SubjectReference,
	_ *model.Pagination, _ model.Consistency,
) (model.ResultStream[model.LookupObjectsItem], error) {
	return &emptyLookupObjectsStream{}, nil
}

func (a *AllowAllRelationsRepository) LookupSubjects(_ context.Context,
	_ model.ResourceReference, _ model.Relation,
	_ model.RepresentationType,
	_ *model.Relation,
	_ *model.Pagination, _ model.Consistency,
) (model.ResultStream[model.LookupSubjectsItem], error) {
	return &emptyLookupSubjectsStream{}, nil
}

func (a *AllowAllRelationsRepository) CreateTuples(_ context.Context, _ []model.RelationsTuple, _ bool, _ *model.FencingCheck,
) (model.TuplesResult, error) {
	return model.NewTuplesResult(model.MinimizeLatencyToken), nil
}

func (a *AllowAllRelationsRepository) DeleteTuples(_ context.Context, _ model.TupleFilter, _ *model.FencingCheck,
) (model.TuplesResult, error) {
	return model.NewTuplesResult(model.MinimizeLatencyToken), nil
}

func (a *AllowAllRelationsRepository) ReadTuples(_ context.Context, _ model.TupleFilter, _ *model.Pagination, _ model.Consistency,
) (model.ResultStream[model.ReadTuplesItem], error) {
	return &emptyReadTuplesStream{}, nil
}

func (a *AllowAllRelationsRepository) AcquireLock(_ context.Context, _ model.LockId) (model.AcquireLockResult, error) {
	return model.NewAcquireLockResult(model.LockToken("")), nil
}
