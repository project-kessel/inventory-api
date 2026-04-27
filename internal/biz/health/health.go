package biz

import (
	"context"

	"github.com/go-kratos/kratos/v2/log"

	"github.com/project-kessel/inventory-api/internal/biz/model"
)

type HealthRepo interface {
	IsBackendAvailable(ctx context.Context) (model.HealthResult, error)
	IsRelationsAvailable(ctx context.Context) (model.HealthResult, error)
}

// HealthUsecase is a Health usecase.
type HealthUsecase struct {
	repo HealthRepo
	log  *log.Helper
}

// New creates a new a Health usecase.
func New(repo HealthRepo, logger log.Logger) *HealthUsecase {
	return &HealthUsecase{repo: repo, log: log.NewHelper(logger)}
}

func (rc *HealthUsecase) IsBackendAvailable(ctx context.Context) (model.HealthResult, error) {
	return rc.repo.IsBackendAvailable(ctx)
}
