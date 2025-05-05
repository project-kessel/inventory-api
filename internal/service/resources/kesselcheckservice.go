package resources

import (
	"context"

	"github.com/project-kessel/inventory-api/internal/biz/usecase/resources"

	"github.com/project-kessel/relations-api/api/kessel/relations/v1beta1"

	pb "github.com/project-kessel/inventory-api/api/kessel/inventory/v1beta1/authz"
	authnapi "github.com/project-kessel/inventory-api/internal/authn/api"
	"github.com/project-kessel/inventory-api/internal/biz/model"
	"github.com/project-kessel/inventory-api/internal/middleware"
)

// TODO: depends on how dynamic resources handles this?
const (
	ResourceType = "integration"
)

type KesselCheckServiceService struct {
	pb.UnimplementedKesselCheckServiceServer

	Ctl *resources.Usecase
}

func NewKesselCheckServiceV1beta1(c *resources.Usecase) *KesselCheckServiceService {
	return &KesselCheckServiceService{
		Ctl: c,
	}
}

func (s *KesselCheckServiceService) Check(ctx context.Context, req *pb.CheckRequest) (*pb.CheckResponse, error) {
	identity, err := middleware.GetIdentity(ctx)
	if err != nil {
		return nil, err
	}

	if resource, err := authzFromRequest(identity, req.Parent); err == nil {
		if resp, err := s.Ctl.Check(ctx, req.GetRelation(), req.Parent.GetType().GetNamespace(), &v1beta1.SubjectReference{
			Relation: req.GetSubject().Relation,
			Subject: &v1beta1.ObjectReference{
				Type: &v1beta1.ObjectType{
					Namespace: req.GetSubject().GetSubject().GetType().GetNamespace(),
					Name:      req.GetSubject().GetSubject().GetType().GetName(),
				},
				Id: req.GetSubject().GetSubject().GetId(),
			},
		}, *resource); err == nil {
			return viewResponseFromAuthzRequest(resp), nil
		} else {
			return nil, err
		}
	} else {
		return nil, err
	}
}

func (s *KesselCheckServiceService) CheckForUpdate(ctx context.Context, req *pb.CheckForUpdateRequest) (*pb.CheckForUpdateResponse, error) {
	identity, err := middleware.GetIdentity(ctx)
	if err != nil {
		return nil, err
	}

	if resource, err := authzFromRequest(identity, req.Parent); err == nil {
		if resp, err := s.Ctl.CheckForUpdate(ctx, req.GetRelation(), req.Parent.Type.GetNamespace(), &v1beta1.SubjectReference{
			Relation: req.GetSubject().Relation,
			Subject: &v1beta1.ObjectReference{
				Type: &v1beta1.ObjectType{
					Namespace: req.GetSubject().GetSubject().GetType().GetNamespace(),
					Name:      req.GetSubject().GetSubject().GetType().GetName(),
				},
				Id: req.GetSubject().GetSubject().GetId(),
			},
		}, *resource); err == nil {
			return updateResponseFromAuthzRequest(resp), nil
		} else {
			return nil, err
		}
	} else {
		return nil, err
	}
}

func authzFromRequest(identity *authnapi.Identity, resource *pb.ObjectReference) (*model.ReporterResourceId, error) {
	return &model.ReporterResourceId{
		LocalResourceId: resource.Id,
		ResourceType:    resource.Type.Name,
		ReporterId:      identity.Principal,
		ReporterType:    identity.Type,
	}, nil
}

func viewResponseFromAuthzRequest(allowed bool) *pb.CheckResponse {
	if allowed {
		return &pb.CheckResponse{Allowed: pb.CheckResponse_ALLOWED_TRUE}
	} else {
		return &pb.CheckResponse{Allowed: pb.CheckResponse_ALLOWED_FALSE}
	}
}

func updateResponseFromAuthzRequest(allowed bool) *pb.CheckForUpdateResponse {
	if allowed {
		return &pb.CheckForUpdateResponse{Allowed: pb.CheckForUpdateResponse_ALLOWED_TRUE}
	} else {
		return &pb.CheckForUpdateResponse{Allowed: pb.CheckForUpdateResponse_ALLOWED_FALSE}
	}
}
