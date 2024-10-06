package k8spolicies

import (
	"context"

	"github.com/project-kessel/inventory-api/api/kessel/inventory/v1beta1/resources"
	pb "github.com/project-kessel/inventory-api/api/kessel/inventory/v1beta1/resources"
	authnapi "github.com/project-kessel/inventory-api/internal/authn/api"
	biz "github.com/project-kessel/inventory-api/internal/biz/resources/k8spolicies"
	"github.com/project-kessel/inventory-api/internal/middleware"
	conv "github.com/project-kessel/inventory-api/internal/service/common"
)

// K8sPoliciesService handles requests for K8s Policies
type K8sPolicyService struct {
	resources.UnimplementedKesselK8SPolicyServiceServer

	Ctl *biz.K8sPolicyUsecase
}

// New creates a new K8sPoliciesService to handle requests for K8s Policies
func New(c *biz.K8sPolicyUsecase) *K8sPolicyService {
	return &K8sPolicyService{
		Ctl: c,
	}
}

func (c *K8sPolicyService) CreateK8SPolicy(ctx context.Context, r *resources.CreateK8SPolicyRequest) (*resources.CreateK8SPolicyResponse, error) {
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

func (c *K8sPolicyService) UpdateK8SPolicy(ctx context.Context, r *resources.UpdateK8SPolicyRequest) (*resources.UpdateK8SPolicyResponse, error) {
	identity, err := middleware.GetIdentity(ctx)
	if err != nil {
		return nil, err
	}

	if h, err := k8sPolicyFromUpdateRequest(r, identity); err == nil {
		// Todo: Update to use the right ID
		if resp, err := c.Ctl.Update(ctx, h, ""); err == nil {
			return updateResponseFromK8sPolicy(resp), nil

		} else {
			return nil, err
		}
	} else {
		return nil, err
	}
}

func (c *K8sPolicyService) DeleteK8SPolicy(ctx context.Context, r *resources.DeleteK8SPolicyRequest) (*resources.DeleteK8SPolicyResponse, error) {
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

func k8sPolicyFromCreateRequest(r *pb.CreateK8SPolicyRequest, identity *authnapi.Identity) (*biz.K8sPolicy, error) {
	var metadata = &pb.Metadata{}
	if r.K8SPolicy.Metadata != nil {
		metadata = r.K8SPolicy.Metadata
	}

	return &biz.K8sPolicy{
		Metadata: *conv.MetadataFromPb(metadata, r.K8SPolicy.ReporterData, identity),
		ResourceData: &biz.K8sPolicyDetail{
			Disabled: r.K8SPolicy.ResourceData.Disabled,
			Severity: r.K8SPolicy.ResourceData.Severity.String(),
		},
	}, nil
}

func createResponseFromK8sPolicy(p *biz.K8sPolicy) *pb.CreateK8SPolicyResponse {
	return &pb.CreateK8SPolicyResponse{}
}

func k8sPolicyFromUpdateRequest(r *pb.UpdateK8SPolicyRequest, identity *authnapi.Identity) (*biz.K8sPolicy, error) {
	var metadata = &pb.Metadata{}
	if r.K8SPolicy.Metadata != nil {
		metadata = r.K8SPolicy.Metadata
	}

	return &biz.K8sPolicy{
		Metadata: *conv.MetadataFromPb(metadata, r.K8SPolicy.ReporterData, identity),
		ResourceData: &biz.K8sPolicyDetail{
			Disabled: r.K8SPolicy.ResourceData.Disabled,
			Severity: r.K8SPolicy.ResourceData.Severity.String(),
		},
	}, nil
}

func updateResponseFromK8sPolicy(p *biz.K8sPolicy) *pb.UpdateK8SPolicyResponse {
	return &pb.UpdateK8SPolicyResponse{}
}

func fromDeleteRequest(r *pb.DeleteK8SPolicyRequest) (string, error) {
	// Todo: Find out what IDs are we going to be using - is it inventory ids? or resources from reporters?
	return r.ReporterData.LocalResourceId, nil
}

func toDeleteResponse() *pb.DeleteK8SPolicyResponse {
	return &pb.DeleteK8SPolicyResponse{}
}
