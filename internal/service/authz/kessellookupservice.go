package service

import (
	"fmt"
	pbv1beta2 "github.com/project-kessel/inventory-api/api/kessel/inventory/v1beta2/authz"
	"github.com/project-kessel/inventory-api/internal/biz/resources"
	kessel "github.com/project-kessel/relations-api/api/kessel/relations/v1beta1"
	"io"
)

type KesselLookupService struct {
	pbv1beta2.UnimplementedKesselLookupServiceServer
	Ctl *resources.Usecase
}

func NewKesselLookupService(c *resources.Usecase) pbv1beta2.KesselLookupServiceServer {
	return &KesselLookupService{
		Ctl: c,
	}
}

func (s *KesselLookupService) LookupResources(
	req *pbv1beta2.LookupResourcesRequest,
	stream pbv1beta2.KesselLookupService_LookupResourcesServer,
) error {
	ctx := stream.Context()
	clientStream, err := s.Ctl.LookupResources(ctx, toLookupResourceRequest(req))
	if err != nil {
		return fmt.Errorf("failed to retrieve resources: %w", err)
	}

	for {
		// Receive next message from the server stream
		resp, err := clientStream.Recv()
		if err == io.EOF {
			// Stream ended successfully
			return nil
		}
		if err != nil {
			return fmt.Errorf("error receiving resource: %w", err)
		}

		// Convert and send the response to the client
		if err := stream.Send(toLookupResourceResponse(resp)); err != nil {
			return fmt.Errorf("error sending resource to client: %w", err)
		}
	}
}

func toLookupResourceRequest(request *pbv1beta2.LookupResourcesRequest) *kessel.LookupResourcesRequest {
	if request == nil {
		return nil
	}
	var pagination *kessel.RequestPagination
	if request.Pagination != nil {
		pagination = &kessel.RequestPagination{
			Limit:             request.Pagination.Limit,
			ContinuationToken: request.Pagination.ContinuationToken,
		}
	}
	return &kessel.LookupResourcesRequest{
		ResourceType: &kessel.ObjectType{
			Namespace: request.ResourceType.Namespace,
			Name:      request.ResourceType.Name,
		},
		Relation: request.Relation,
		Subject: &kessel.SubjectReference{
			Relation: request.Subject.Relation,
			Subject: &kessel.ObjectReference{
				Type: &kessel.ObjectType{
					Name:      request.Subject.Subject.Type.Name,
					Namespace: request.Subject.Subject.Type.Namespace,
				},
				Id: request.Subject.Subject.Id,
			},
		},
		Pagination: pagination,
	}
}

func toLookupResourceResponse(response *kessel.LookupResourcesResponse) *pbv1beta2.LookupResourcesResponse {
	return &pbv1beta2.LookupResourcesResponse{
		Resource: &pbv1beta2.ObjectReference{
			Type: &pbv1beta2.ObjectType{
				Namespace: response.Resource.Type.Namespace,
				Name:      response.Resource.Type.Name,
			},
			Id: response.Resource.Id,
		},
		Pagination: &pbv1beta2.ResponsePagination{
			ContinuationToken: response.Pagination.ContinuationToken,
		},
	}
}
