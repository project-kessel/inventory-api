package policies

import (
	"context"

	"github.com/go-kratos/kratos/v2/log"
)

type Policy struct {
	Hello string
}

// PolicyRepo is a Policy repo.
type PolicyRepo interface {
	Save(context.Context, *Policy) (*Policy, error)
	Update(context.Context, *Policy) (*Policy, error)
	FindByID(context.Context, int64) (*Policy, error)
	ListAll(context.Context) ([]*Policy, error)
}

// PolicyUsecase is a Policy usecase.
type PolicyUsecase struct {
	repo PolicyRepo
	log  *log.Helper
}

// New new a Policy usecase.
func New(repo PolicyRepo, logger log.Logger) *PolicyUsecase {
	return &PolicyUsecase{repo: repo, log: log.NewHelper(logger)}
}

// CreatePolicy creates a Policy, and returns the new Policy.
func (uc *PolicyUsecase) CreatePolicy(ctx context.Context, h *Policy) (*Policy, error) {
	uc.log.WithContext(ctx).Infof("CreatePolicy: %v", h.Hello)
	return uc.repo.Save(ctx, h)
}
