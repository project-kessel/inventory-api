package k8sclusters

import (
	"context"

	"github.com/go-kratos/kratos/v2/log"
)

const (
	ResourceType = "k8s-cluster"
)

// K8sClusterRepo is a K8sCluster repo.
type K8sClusterRepo interface {
	Save(context.Context, *K8SCluster) (*K8SCluster, error)
	Update(context.Context, *K8SCluster, string) (*K8SCluster, error)
	Delete(context.Context, string) error
	FindByID(context.Context, string) (*K8SCluster, error)
	ListAll(context.Context) ([]*K8SCluster, error)
}

// K8sClusterUsecase is a K8sCluster usecase.
type K8sClusterUsecase struct {
	repo K8sClusterRepo
	log  *log.Helper
}

// New creates a new a K8sCluster usecase.
func New(repo K8sClusterRepo, logger log.Logger) *K8sClusterUsecase {
	return &K8sClusterUsecase{repo: repo, log: log.NewHelper(logger)}
}

// Create creates a K8sCluster and returns the new K8sCluster.
func (uc *K8sClusterUsecase) Create(ctx context.Context, c *K8SCluster) (*K8SCluster, error) {
	if ret, err := uc.repo.Save(ctx, c); err != nil {
		return nil, err
	} else {
		return ret, nil
	}
}

// Update updates a K8SCluster in the repository and returns the updated K8SCluster.
func (uc *K8sClusterUsecase) Update(ctx context.Context, h *K8SCluster, id string) (*K8SCluster, error) {
	if ret, err := uc.repo.Update(ctx, h, id); err != nil {
		return nil, err
	} else {
		return ret, nil
	}
}

// Delete deletes a K8SCluster from the repository.
func (uc *K8sClusterUsecase) Delete(ctx context.Context, id string) error {
	if err := uc.repo.Delete(ctx, id); err != nil {
		return err
	} else {
		return nil
	}
}
