package k8sclusters

import (
	"context"
	"fmt"
	"strings"

	"github.com/go-kratos/kratos/v2/errors"

	"github.com/project-kessel/inventory-api/api/kessel/inventory/v1beta1/resources"
	pb "github.com/project-kessel/inventory-api/api/kessel/inventory/v1beta1/resources"
	authnapi "github.com/project-kessel/inventory-api/internal/authn/api"
	biz "github.com/project-kessel/inventory-api/internal/biz/k8sclusters"
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
	if !strings.EqualFold(r.K8SCluster.Metadata.ResourceType, biz.ResourceType) {
		return nil, errors.BadRequest("BADREQUEST", fmt.Sprintf("incorrect resource type: expected %s", biz.ResourceType))
	}

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
	return nil, nil
}

func (c *K8sClustersService) DeleteK8SCluster(ctx context.Context, r *resources.DeleteK8SClusterRequest) (*resources.DeleteK8SClusterResponse, error) {
	return nil, nil
}

func k8sClusterFromCreateRequest(r *pb.CreateK8SClusterRequest, identity *authnapi.Identity) (*biz.K8SCluster, error) {
	if identity.Principal != r.K8SCluster.ReporterData.ReporterInstanceId {
		msg := fmt.Sprintf("Reporter identity must match the provided reporter instance identity: %s != %s", identity.Principal, r.K8SCluster.ReporterData.ReporterInstanceId)
		return nil, errors.Forbidden("FORBIDDEN", msg)
	}

	return &biz.K8SCluster{
		Metadata:     *conv.MetadataFromPb(r.K8SCluster.Metadata, r.K8SCluster.ReporterData, identity),
		ResourceData: resourceDataFromPb(r.K8SCluster.ResourceData),
	}, nil
}

func createResponseFromK8sCluster(c *biz.K8SCluster) *pb.CreateK8SClusterResponse {
	return &pb.CreateK8SClusterResponse{}
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

func pbResourceDataFromModel(m *biz.K8SClusterDetail) *pb.K8SClusterDetail {
	var nodes []*pb.K8SClusterDetailNodesInner
	for _, n := range m.Nodes {
		var labels []*pb.ResourceLabel
		for _, l := range n.Labels {
			labels = append(labels, &pb.ResourceLabel{Key: l.Key, Value: l.Value})
		}

		nodes = append(nodes, &pb.K8SClusterDetailNodesInner{
			Name:   n.Name,
			Cpu:    n.Cpu,
			Memory: n.Memory,
			Labels: labels,
		})
	}

	// TODO: Error handling if the string lookups fail in the pb maps
	rd := &pb.K8SClusterDetail{
		ExternalClusterId: m.ExternalClusterId,
		ClusterStatus:     pb.K8SClusterDetail_ClusterStatus(pb.K8SClusterDetail_ClusterStatus_value[m.ClusterStatus]),

		KubeVersion: m.KubeVersion,
		KubeVendor:  pb.K8SClusterDetail_KubeVendor(pb.K8SClusterDetail_KubeVendor_value[m.KubeVendor]),

		VendorVersion: m.VendorVersion,
		CloudPlatform: pb.K8SClusterDetail_CloudPlatform(pb.K8SClusterDetail_CloudPlatform_value[m.CloudPlatform]),

		Nodes: nodes,
	}
	return rd
}
