package k8sclusters

import (
	"context"
	"github.com/project-kessel/inventory-api/api/kessel/inventory/v1beta1/resources"
	pb "github.com/project-kessel/inventory-api/api/kessel/inventory/v1beta1/resources"
	authnapi "github.com/project-kessel/inventory-api/internal/authn/api"
	biz "github.com/project-kessel/inventory-api/internal/biz/resources/k8sclusters"
	"github.com/project-kessel/inventory-api/internal/middleware"
	conv "github.com/project-kessel/inventory-api/internal/service/common"
)

// K8sClustersService handles requests for k8s clusters
type K8sClustersService struct {
	resources.UnimplementedKesselK8SClusterServiceServer

	Ctl *biz.K8sClusterUsecase
}

// New creates a new K8sClusterService to handle requests for k8s clusters
func New(c *biz.K8sClusterUsecase) *K8sClustersService {
	return &K8sClustersService{
		Ctl: c,
	}
}

func (c *K8sClustersService) CreateK8SCluster(ctx context.Context, r *resources.CreateK8SClusterRequest) (*resources.CreateK8SClusterResponse, error) {
	identity, err := middleware.GetIdentity(ctx)
	if err != nil {
		return nil, err
	}

	if k, err := k8sClusterFromCreateRequest(r, identity); err == nil {
		k.Metadata.ResourceType = biz.ResourceType
		if resp, err := c.Ctl.Create(ctx, k); err == nil {
			return createResponseFromK8sCluster(resp), nil
		} else {
			return nil, err
		}
	} else {
		return nil, err
	}
}

func (c *K8sClustersService) UpdateK8SCluster(ctx context.Context, r *resources.UpdateK8SClusterRequest) (*resources.UpdateK8SClusterResponse, error) {
	identity, err := middleware.GetIdentity(ctx)
	if err != nil {
		return nil, err
	}

	if k, err := k8sClusterFromUpdateRequest(r, identity); err == nil {
		k.Metadata.ResourceType = biz.ResourceType
		// Todo: Update to use the right ID
		if resp, err := c.Ctl.Update(ctx, k, ""); err == nil {
			return updateResponseFromK8sCluster(resp), nil
		} else {
			return nil, err
		}
	} else {
		return nil, err
	}
}

func (c *K8sClustersService) DeleteK8SCluster(ctx context.Context, r *resources.DeleteK8SClusterRequest) (*resources.DeleteK8SClusterResponse, error) {
	if input, err := fromDeleteRequest(r); err == nil {
		if err := c.Ctl.Delete(ctx, input); err == nil {
			return toDeleteResponse(), nil
		} else {
			return nil, err
		}
	} else {
		return nil, err
	}
}

func k8sClusterFromCreateRequest(r *pb.CreateK8SClusterRequest, identity *authnapi.Identity) (*biz.K8SCluster, error) {
	return &biz.K8SCluster{
		Metadata:     *conv.MetadataFromPb(r.K8SCluster.Metadata, r.K8SCluster.ReporterData, identity),
		ResourceData: resourceDataFromPb(r.K8SCluster.ResourceData),
	}, nil
}

func createResponseFromK8sCluster(c *biz.K8SCluster) *pb.CreateK8SClusterResponse {
	return &pb.CreateK8SClusterResponse{}
}

func k8sClusterFromUpdateRequest(r *pb.UpdateK8SClusterRequest, identity *authnapi.Identity) (*biz.K8SCluster, error) {
	return &biz.K8SCluster{
		Metadata:     *conv.MetadataFromPb(r.K8SCluster.Metadata, r.K8SCluster.ReporterData, identity),
		ResourceData: resourceDataFromPb(r.K8SCluster.ResourceData),
	}, nil
}

func updateResponseFromK8sCluster(c *biz.K8SCluster) *pb.UpdateK8SClusterResponse {
	return &pb.UpdateK8SClusterResponse{}
}

func fromDeleteRequest(r *pb.DeleteK8SClusterRequest) (string, error) {
	// Todo: Find out what IDs are we going to be using - is it inventory ids? or resources from reporters?
	return r.ReporterData.LocalResourceId, nil
}

func toDeleteResponse() *pb.DeleteK8SClusterResponse {
	return &pb.DeleteK8SClusterResponse{}
}

func resourceDataFromPb(r *pb.K8SClusterDetail) *biz.K8SClusterDetail {
	var nodes []biz.Node
	for _, n := range r.Nodes {
		var labels []biz.NodeLabel
		for _, l := range n.Labels {
			labels = append(labels, biz.NodeLabel{Key: l.Key, Value: l.Value})
		}

		nodes = append(nodes, biz.Node{
			Name:   n.Name,
			Cpu:    n.Cpu,
			Memory: n.Memory,
			Labels: labels,
		})
	}

	rd := &biz.K8SClusterDetail{
		ExternalClusterId: r.ExternalClusterId,
		ClusterStatus:     r.ClusterStatus.String(),

		KubeVersion: r.KubeVersion,
		KubeVendor:  r.KubeVendor.String(),

		VendorVersion: r.VendorVersion,
		CloudPlatform: r.CloudPlatform.String(),

		Nodes: nodes,
	}
	return rd
}
