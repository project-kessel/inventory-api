package k8spolicies

import (
	"context"

	"github.com/project-kessel/inventory-api/internal/biz/model"
	"github.com/project-kessel/inventory-api/internal/biz/resources"

	pb "github.com/project-kessel/inventory-api/api/kessel/inventory/v1beta1/resources"
	authnapi "github.com/project-kessel/inventory-api/internal/authn/api"
	"github.com/project-kessel/inventory-api/internal/middleware"
	conv "github.com/project-kessel/inventory-api/internal/service/common"
)

const (
	ResourceType = "k8s_policy"
)

// K8sPoliciesService handles requests for K8s Policies
type K8sPolicyService struct {
	pb.UnimplementedKesselK8SPolicyServiceServer

	Ctl *resources.Usecase
}

// New creates a new K8sPoliciesService to handle requests for K8s Policies
func New(c *resources.Usecase) *K8sPolicyService {
	return &K8sPolicyService{
		Ctl: c,
	}
}

func (c *K8sPolicyService) CreateK8SPolicy(ctx context.Context, r *pb.CreateK8SPolicyRequest) (*pb.CreateK8SPolicyResponse, error) {
	identity, err := middleware.GetIdentity(ctx)
	if err != nil {
		return nil, err
	}

	if h, err := k8sPolicyFromCreateRequest(r, identity); err == nil {
		if resp, err := c.Ctl.Create(ctx, h); err == nil {
			return createResponseFromK8sPolicy(resp), nil

		} else {
			return nil, err
		}
	} else {
		return nil, err
	}
}

func (c *K8sPolicyService) UpdateK8SPolicy(ctx context.Context, r *pb.UpdateK8SPolicyRequest) (*pb.UpdateK8SPolicyResponse, error) {
	identity, err := middleware.GetIdentity(ctx)
	if err != nil {
		return nil, err
	}

	if h, err := k8sPolicyFromUpdateRequest(r, identity); err == nil {
		// Todo: Update to use the right ID
		if resp, err := c.Ctl.Update(ctx, h, model.ReporterResourceIdFromResource(h)); err == nil {
			return updateResponseFromK8sPolicy(resp), nil

		} else {
			return nil, err
		}
	} else {
		return nil, err
	}
}

func (c *K8sPolicyService) DeleteK8SPolicy(ctx context.Context, r *pb.DeleteK8SPolicyRequest) (*pb.DeleteK8SPolicyResponse, error) {
	identity, err := middleware.GetIdentity(ctx)
	if err != nil {
		return nil, err
	}

	if resourceId, err := fromDeleteRequest(r, identity); err == nil {
		if err := c.Ctl.Delete(ctx, resourceId); err == nil {
			return toDeleteResponse(), nil
		} else {
			return nil, err
		}
	} else {
		return nil, err
	}
}

func k8sPolicyFromCreateRequest(r *pb.CreateK8SPolicyRequest, identity *authnapi.Identity) (*model.Resource, error) {
	resourceData, err := conv.ToJsonObject(r.K8SPolicy.ResourceData)
	if err != nil {
		return nil, err
	}

	return conv.ResourceFromPb(ResourceType, identity.Principal, resourceData, r.K8SPolicy.Metadata, r.K8SPolicy.ReporterData), nil
}

func createResponseFromK8sPolicy(p *model.Resource) *pb.CreateK8SPolicyResponse {
	return &pb.CreateK8SPolicyResponse{}
}

func k8sPolicyFromUpdateRequest(r *pb.UpdateK8SPolicyRequest, identity *authnapi.Identity) (*model.Resource, error) {
	resourceData, err := conv.ToJsonObject(r.K8SPolicy.ResourceData)
	if err != nil {
		return nil, err
	}

	return conv.ResourceFromPb(ResourceType, identity.Principal, resourceData, r.K8SPolicy.Metadata, r.K8SPolicy.ReporterData), nil
}

func updateResponseFromK8sPolicy(p *model.Resource) *pb.UpdateK8SPolicyResponse {
	return &pb.UpdateK8SPolicyResponse{}
}

func fromDeleteRequest(r *pb.DeleteK8SPolicyRequest, identity *authnapi.Identity) (model.ReporterResourceId, error) {
	return conv.ReporterResourceIdFromPb(ResourceType, identity.Principal, r.ReporterData), nil
}

func toDeleteResponse() *pb.DeleteK8SPolicyResponse {
	return &pb.DeleteK8SPolicyResponse{}
}
