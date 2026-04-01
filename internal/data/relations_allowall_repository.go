package data

import (
	"context"
	"io"

	"github.com/go-kratos/kratos/v2/log"

	"github.com/project-kessel/inventory-api/internal/biz/model"
)

type allowAllRelationsRepository struct {
	logger *log.Helper
}

var _ model.RelationsRepository = &allowAllRelationsRepository{}

func newAllowAllRelationsRepository(logger *log.Helper) *allowAllRelationsRepository {
	logger.Info("Using relations repository: allow-all")
	return &allowAllRelationsRepository{logger: logger}
}

func (r *allowAllRelationsRepository) Health(_ context.Context) error {
	return nil
}

func (r *allowAllRelationsRepository) Check(_ context.Context, _ model.ReporterResourceKey, _ model.Relation,
	_ model.SubjectReference, _ model.Consistency) (bool, model.ConsistencyToken, error) {
	return true, "", nil
}

func (r *allowAllRelationsRepository) CheckForUpdate(_ context.Context, _ model.ReporterResourceKey, _ model.Relation,
	_ model.SubjectReference) (bool, model.ConsistencyToken, error) {
	return true, "", nil
}

func (r *allowAllRelationsRepository) CheckBulk(_ context.Context, items []model.CheckItem,
	_ model.Consistency) ([]model.CheckBulkResultItem, model.ConsistencyToken, error) {
	results := make([]model.CheckBulkResultItem, len(items))
	for i := range items {
		results[i] = model.CheckBulkResultItem{Allowed: true}
	}
	return results, "", nil
}

func (r *allowAllRelationsRepository) CheckForUpdateBulk(_ context.Context, items []model.CheckItem) ([]model.CheckBulkResultItem, model.ConsistencyToken, error) {
	results := make([]model.CheckBulkResultItem, len(items))
	for i := range items {
		results[i] = model.CheckBulkResultItem{Allowed: true}
	}
	return results, "", nil
}

func (r *allowAllRelationsRepository) LookupResources(_ context.Context, _ model.LookupResourcesQuery) (model.LookupResourcesIterator, error) {
	return &emptyLookupIterator{}, nil
}

func (r *allowAllRelationsRepository) CreateTuples(_ context.Context, _ []model.RelationsTuple, _ bool,
	_, _ string) (model.ConsistencyToken, error) {
	return "", nil
}

func (r *allowAllRelationsRepository) DeleteTuples(_ context.Context, _ []model.RelationsTuple,
	_, _ string) (model.ConsistencyToken, error) {
	return "", nil
}

func (r *allowAllRelationsRepository) AcquireLock(_ context.Context, _ string) (string, error) {
	return "", nil
}

type emptyLookupIterator struct{}

func (e *emptyLookupIterator) Next() (*model.LookupResourceResult, error) {
	return nil, io.EOF
}
