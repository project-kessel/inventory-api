package getresources

import (
	"context"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/project-kessel/inventory-api/internal/biz/model"
)

type ResourceRepository interface {
	FindByID(context.Context, uint64) (*model.Resource, error)
}

type Usecase struct {
	repository ResourceRepository
	log        *log.Helper
}

func New(repository ResourceRepository, logger log.Logger) *Usecase {
	return &Usecase{
		repository: repository,
		log:        log.NewHelper(logger),
	}
}

func (uc *Usecase) FindById(ctx context.Context, resourceId uint64) (*model.Resource, error) {
	return uc.repository.FindByID(ctx, resourceId)
}
