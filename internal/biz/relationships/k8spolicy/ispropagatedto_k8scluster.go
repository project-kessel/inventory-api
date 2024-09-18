package k8spolicy

import (
	"context"
	"github.com/project-kessel/inventory-api/internal/biz/common"

	"github.com/go-kratos/kratos/v2/log"
)

const (
	RelationType = "k8s-policy_ispropagatedto_k8s-cluster"
)

type K8SPolicyIsPropagatedToK8SCluster struct {
	// Kessel Asset Inventory generated identifier.
	ID int64 `gorm:"primaryKey"`

	MetadataID int64
	Metadata   common.RelationshipMetadata

	Status string
}

// K8SPolicyIsPropagatedToK8SClusterRepo is a K8SPolicyIsPropagatedToK8SCluster repo.
type K8SPolicyIsPropagatedToK8SClusterRepo interface {
	Save(context.Context, *K8SPolicyIsPropagatedToK8SCluster) (*K8SPolicyIsPropagatedToK8SCluster, error)
	Update(context.Context, *K8SPolicyIsPropagatedToK8SCluster, string) (*K8SPolicyIsPropagatedToK8SCluster, error)
	Delete(context.Context, string) error
	FindByID(context.Context, string) (*K8SPolicyIsPropagatedToK8SCluster, error)
	ListAll(context.Context) ([]*K8SPolicyIsPropagatedToK8SCluster, error)
}

// K8SPolicyIsPropagatedToK8SClusterUsecase is a K8SPolicyIsPropagatedToK8SCluster usecase.
type K8SPolicyIsPropagatedToK8SClusterUsecase struct {
	repo K8SPolicyIsPropagatedToK8SClusterRepo
	log  *log.Helper
}

// New new a K8SPolicyIsPropagatedToK8SCluster usecase.
func New(repo K8SPolicyIsPropagatedToK8SClusterRepo, logger log.Logger) *K8SPolicyIsPropagatedToK8SClusterUsecase {
	return &K8SPolicyIsPropagatedToK8SClusterUsecase{repo: repo, log: log.NewHelper(logger)}
}

// Create creates a K8SPolicyIsPropagatedToK8SCluster, and returns the new K8SPolicyIsPropagatedToK8SCluster.
func (uc *K8SPolicyIsPropagatedToK8SClusterUsecase) Create(ctx context.Context, r *K8SPolicyIsPropagatedToK8SCluster) (*K8SPolicyIsPropagatedToK8SCluster, error) {
	if ret, err := uc.repo.Save(ctx, r); err != nil {
		return nil, err
	} else {
		uc.log.WithContext(ctx).Infof("Create K8SPolicyIsPropagatedToK8SCluster: %v", r.ID)
		return ret, nil
	}
}

// Update updates a CreateK8SPolicyIsPropagatedToK8SCluster in the repository and returns the updated CreateK8SPolicyIsPropagatedToK8SCluster.
func (uc *K8SPolicyIsPropagatedToK8SClusterUsecase) Update(ctx context.Context, h *K8SPolicyIsPropagatedToK8SCluster, id string) (*K8SPolicyIsPropagatedToK8SCluster, error) {
	if ret, err := uc.repo.Update(ctx, h, id); err != nil {
		return nil, err
	} else {
		uc.log.WithContext(ctx).Infof("Update K8SPolicyIsPropagatedToK8SCluster: %v", h.ID)
		return ret, nil
	}
}

// Delete deletes a K8SPolicyIsPropagatedToK8SCluster in the repository.
func (uc *K8SPolicyIsPropagatedToK8SClusterUsecase) Delete(ctx context.Context, id string) error {
	if err := uc.repo.Delete(ctx, id); err != nil {
		return err
	} else {
		uc.log.WithContext(ctx).Infof("Delete K8SPolicyIsPropagatedToK8SCluster: %v", id)
		return nil
	}
}
