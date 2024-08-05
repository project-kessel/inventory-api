package k8sclusters

import (
	"context"

	"github.com/project-kessel/inventory-api/api/kessel/inventory/v1beta1"
	controller "github.com/project-kessel/inventory-api/internal/biz/k8sclusters"
)

// K8sClustersService handles requests for k8s clusters
type K8sClustersService struct {
	v1beta1.UnimplementedKesselK8SClusterServiceServer

	Controller *controller.K8sClusterUsecase
}

// New creates a new K8sClusterService to handle requests for k8s clusters
func New(c *controller.K8sClusterUsecase) *K8sClustersService {
	return &K8sClustersService{
		Controller: c,
	}
}

func (c *K8sClustersService) CreateK8SCluster(ctx context.Context, r *v1beta1.CreateK8SClusterRequest) (*v1beta1.CreateK8SClusterResponse, error) {
	return nil, nil
}

func (c *K8sClustersService) UpdateK8SCluster(ctx context.Context, r *v1beta1.UpdateK8SClusterRequest) (*v1beta1.UpdateK8SClusterResponse, error) {
	return nil, nil
}

func (c *K8sClustersService) DeleteK8SCluster(ctx context.Context, r *v1beta1.DeleteK8SClusterRequest) (*v1beta1.DeleteK8SClusterResponse, error) {
	return nil, nil
}
