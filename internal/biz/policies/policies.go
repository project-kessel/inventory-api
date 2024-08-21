package policies

import (
	"context"

	"github.com/go-kratos/kratos/v2/log"
)

const (
	ResourceType = "policy"
)

// PolicyRepo is a Policy repo.
type PolicyRepo interface {
	Save(context.Context, *Policy) (*Policy, error)
	Update(context.Context, *Policy, string) (*Policy, error)
	Delete(context.Context, string) error
	FindByID(context.Context, string) (*Policy, error)
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

// Create creates a Policy, and returns the new Policy.
func (uc *PolicyUsecase) Create(ctx context.Context, p *Policy) (*Policy, error) {
	uc.log.WithContext(ctx).Infof("Create Policy: %v", p)
	return uc.repo.Save(ctx, p)
}

// Update updates a Policy in the repository and returns the updated Policy.
func (uc *PolicyUsecase) Update(ctx context.Context, p *Policy, id string) (*Policy, error) {
	if ret, err := uc.repo.Update(ctx, p, id); err != nil {
		return nil, err
	} else {
		uc.log.WithContext(ctx).Infof("Update Policy: %v", p.ID)
		return ret, nil
	}
}

// Delete deletes a Policy in the repository.
func (uc *PolicyUsecase) Delete(ctx context.Context, id string) error {
	if err := uc.repo.Delete(ctx, id); err != nil {
		return err
	} else {
		uc.log.WithContext(ctx).Infof("Delete Policy: %v", id)
		return nil
	}
}
