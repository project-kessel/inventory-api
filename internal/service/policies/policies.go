package policies

import (
	"context"

	"github.com/go-kratos/kratos/v2/errors"
	"github.com/project-kessel/inventory-api/api/kessel/inventory/v1beta1"
	pb "github.com/project-kessel/inventory-api/api/kessel/inventory/v1beta1"
	authnapi "github.com/project-kessel/inventory-api/internal/authn/api"
	biz "github.com/project-kessel/inventory-api/internal/biz/policies"
	"github.com/project-kessel/inventory-api/internal/middleware"
	conv "github.com/project-kessel/inventory-api/internal/service/common"
)

// PoliciesService handles requests for RHEL hosts
type PoliciesService struct {
	v1beta1.UnimplementedKesselPolicyServiceServer

	Ctl *biz.PolicyUsecase
}

// New creates a new PoliciesService to handle requests for RHEL hosts
func New(c *biz.PolicyUsecase) *PoliciesService {
	return &PoliciesService{
		Ctl: c,
	}
}

func (c *PoliciesService) CreatePolicy(ctx context.Context, r *v1beta1.CreatePolicyRequest) (*v1beta1.CreatePolicyResponse, error) {
	if err := r.ValidateAll(); err != nil {
		return nil, errors.BadRequest("BADREQUEST", err.Error())
	}

	identity, err := middleware.GetIdentity(ctx)
	if err != nil {
		return nil, err
	}

	if h, err := policyFromCreateRequest(r, identity); err == nil {
		if resp, err := c.Ctl.Create(ctx, h); err == nil {
			return createResponseFromPolicy(resp), nil

		} else {
			return nil, err
		}
	} else {
		return nil, err
	}
}

func (c *PoliciesService) UpdatePolicy(ctx context.Context, r *v1beta1.UpdatePolicyRequest) (*v1beta1.UpdatePolicyResponse, error) {
	return nil, nil
}

func (c *PoliciesService) DeletePolicy(ctx context.Context, r *v1beta1.DeletePolicyRequest) (*v1beta1.DeletePolicyResponse, error) {
	return nil, nil
}

func policyFromCreateRequest(r *pb.CreatePolicyRequest, identity *authnapi.Identity) (*biz.Policy, error) {
	var metadata = &pb.Metadata{}
	if r.Policy.Metadata != nil {
		metadata = r.Policy.Metadata
	}

	return &biz.Policy{
		Metadata: *conv.MetadataFromPb(metadata, r.Policy.ReporterData, identity),
		ResourceData: &biz.PolicyDetail{
			Disabled: r.Policy.ResourceData.Disabled,
			Severity: r.Policy.ResourceData.Severity.String(),
		},
	}, nil
}

func createResponseFromPolicy(p *biz.Policy) *pb.CreatePolicyResponse {
	// TODO: Error handling if the string lookups fail in the pb maps
	return &pb.CreatePolicyResponse{
		Policy: &pb.Policy{
			Metadata: conv.MetadataFromModel(&p.Metadata),
			ResourceData: &pb.PolicyDetail{
				Disabled: p.ResourceData.Disabled,
				Severity: pb.PolicyDetail_Severity(pb.PolicyDetail_Severity_value[p.ResourceData.Severity]),
			},
		},
	}
}
