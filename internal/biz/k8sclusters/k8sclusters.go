package k8sclusters

import (
	"context"

	"github.com/go-kratos/kratos/v2/log"
)

type K8sCluster struct {
	Hello string
}

// K8sClusterRepo is a K8sCluster repo.
type K8sClusterRepo interface {
	Save(context.Context, *K8sCluster) (*K8sCluster, error)
	Update(context.Context, *K8sCluster) (*K8sCluster, error)
	FindByID(context.Context, int64) (*K8sCluster, error)
	ListAll(context.Context) ([]*K8sCluster, error)
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
func (uc *K8sClusterUsecase) Create(ctx context.Context, c *K8sCluster) (*K8sCluster, error) {
	uc.log.WithContext(ctx).Infof("CreateK8sCluster: %v", c.Hello)
	return uc.repo.Save(ctx, c)
}
