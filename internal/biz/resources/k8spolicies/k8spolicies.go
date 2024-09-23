package k8spolicies

import (
	"context"

	"github.com/go-kratos/kratos/v2/log"
)

const (
	ResourceType = "k8s-policy"
)

// PolicyRepo is a Policy repo.
type K8sPolicyRepo interface {
	Save(context.Context, *K8sPolicy) (*K8sPolicy, error)
	Update(context.Context, *K8sPolicy, string) (*K8sPolicy, error)
	Delete(context.Context, string) error
	FindByID(context.Context, string) (*K8sPolicy, error)
	ListAll(context.Context) ([]*K8sPolicy, error)
}

// K8sPolicyUsecase is a K8s Policy usecase.
type K8sPolicyUsecase struct {
	repo K8sPolicyRepo
	log  *log.Helper
}

// New new a Policy usecase.
func New(repo K8sPolicyRepo, logger log.Logger) *K8sPolicyUsecase {
	return &K8sPolicyUsecase{repo: repo, log: log.NewHelper(logger)}
}

// Create creates a K8s-Policy, and returns the new K8s-Policy.
func (uc *K8sPolicyUsecase) Create(ctx context.Context, p *K8sPolicy) (*K8sPolicy, error) {
	uc.log.WithContext(ctx).Infof("Create K8s-Policy: %v", p)
	return uc.repo.Save(ctx, p)
}

// Update updates a K8s-Policy in the repository and returns the updated K8s-Policy.
func (uc *K8sPolicyUsecase) Update(ctx context.Context, p *K8sPolicy, id string) (*K8sPolicy, error) {
	if ret, err := uc.repo.Update(ctx, p, id); err != nil {
		return nil, err
	} else {
		uc.log.WithContext(ctx).Infof("Update K8s-Policy: %v", p.ID)
		return ret, nil
	}
}

// Delete deletes a K8s-Policy in the repository.
func (uc *K8sPolicyUsecase) Delete(ctx context.Context, id string) error {
	if err := uc.repo.Delete(ctx, id); err != nil {
		return err
	} else {
		uc.log.WithContext(ctx).Infof("Delete K8s-Policy: %v", id)
		return nil
	}
}
