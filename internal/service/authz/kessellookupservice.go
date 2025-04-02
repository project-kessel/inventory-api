package service

import (
	"context"
	pb "github.com/project-kessel/inventory-api/api/kessel/inventory/v1beta1/authz"

	pbv1beta2 "github.com/project-kessel/inventory-api/api/kessel/inventory/v1beta2/authz"
	authnapi "github.com/project-kessel/inventory-api/internal/authn/api"
	"github.com/project-kessel/inventory-api/internal/biz/model"
	"github.com/project-kessel/inventory-api/internal/biz/resources"
	"github.com/project-kessel/inventory-api/internal/middleware"
	"github.com/project-kessel/relations-api/api/kessel/relations/v1beta1"
)

type KesselLookupService struct {
	pbv1beta2.UnimplementedKesselLookupServiceServer
	Ctl *resources.Usecase
}

func NewKesselLookupService(c *resources.Usecase) *KesselLookupService {
	return &KesselLookupService{
		Ctl: c,
	}
}

func (s *KesselLookupService) LookupResources(ctx context.Context, req *pbv1beta2.LookupResourcesRequest) (*pbv1beta2.LookupResourcesResponse, error) {
	identity, err := middleware.GetIdentity(ctx)
	if err != nil {
		return nil, err
	}

	if resource, err := requestToLookup(identity, req.Parent); err == nil {
		if resp, err := s.Ctl.LookupResources(ctx, req.GetRelation(), req.Parent.Type.GetNamespace(), &v1beta1.SubjectReference{
			Relation: req.GetSubject().Relation,
			Subject: &v1beta1.ObjectReference{
				Type: &v1beta1.ObjectType{
					Namespace: req.GetSubject().GetSubject().GetType().GetNamespace(),
					Name:      req.GetSubject().GetSubject().GetType().GetName(),
				},
				Id: req.GetSubject().GetSubject().GetId(),
			},
		}, *resource); err == nil {
			return lookupResourceResponse(resp), nil
		} else {
			return nil, err
		}
	} else {
		return nil, err
	}
}

func requestToLookup(identity *authnapi.Identity, resource *pb.ObjectReference) (*model.ReporterResourceId, error) {
	return &model.ReporterResourceId{
		LocalResourceId: resource.Id,
		ResourceType:    resource.Type.Name,
		ReporterId:      identity.Principal,
		ReporterType:    identity.Type,
	}, nil
}

func lookupResourceResponse(allowed bool) *pbv1beta2.LookupResourcesResponse {
	if allowed {
		return &pbv1beta2.LookupResourcesResponse{Allowed: pb.CheckResponse_ALLOWED_TRUE}
	} else {
		return &pbv1beta2.LookupResourcesResponse{Allowed: pb.CheckResponse_ALLOWED_FALSE}
	}
}
