package biz

import (
	"context"

	"github.com/go-kratos/kratos/v2/log"
	pb "github.com/project-kessel/inventory-api/api/kessel/inventory/v1"
)

type HealthRepo interface {
	IsBackendAvailable(ctx context.Context) (*pb.GetReadyzResponse, error)
	IsRelationsAvailable(ctx context.Context) (*pb.GetReadyzResponse, error)
}

// HealthUsecase is a Health usecase.
type HealthUsecase struct {
	repo               HealthRepo
	log                *log.Helper
	disablePersistence bool
}

// New creates a new a Health usecase.
func New(repo HealthRepo, logger log.Logger, disablePersistence bool) *HealthUsecase {
	return &HealthUsecase{repo: repo, log: log.NewHelper(logger), disablePersistence: disablePersistence}
}

func (rc *HealthUsecase) IsBackendAvailable(ctx context.Context) (*pb.GetReadyzResponse, error) {
	if rc.disablePersistence {
		return rc.repo.IsRelationsAvailable(ctx)
	}
	return rc.repo.IsBackendAvailable(ctx)
}
