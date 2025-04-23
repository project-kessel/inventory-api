package resources

import (
	"fmt"
	pbv1beta2 "github.com/project-kessel/inventory-api/api/kessel/inventory/v1beta2"
	"github.com/project-kessel/inventory-api/internal/biz/resources"
	kessel "github.com/project-kessel/relations-api/api/kessel/relations/v1beta1"
	"io"
)

type KesselLookupService struct {
	pbv1beta2.UnimplementedKesselLookupServiceServer
	Ctl *resources.Usecase
}

func NewKesselLookupServiceV1beta2(c *resources.Usecase) pbv1beta2.KesselLookupServiceServer {
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
			Namespace: request.ObjectType.GetReporterType(),
			Name:      request.ObjectType.GetResourceType(),
		},
		Relation: request.Relation,
		Subject: &kessel.SubjectReference{
			Relation: request.Subject.Relation,
			Subject: &kessel.ObjectReference{
				Type: &kessel.ObjectType{
					Name:      request.Subject.Resource.GetResourceType(),
					Namespace: request.Subject.Resource.GetReporter().GetType(),
				},
				Id: request.Subject.Resource.GetResourceId(),
			},
		},
		Pagination: pagination,
	}
}

func toLookupResourceResponse(response *kessel.LookupResourcesResponse) *pbv1beta2.LookupResourcesResponse {
	return &pbv1beta2.LookupResourcesResponse{
		Object: &pbv1beta2.ResourceReference{
			Reporter: &pbv1beta2.ReporterReference{
				Type: response.Resource.Type.Namespace,
			},
			ResourceId: response.Resource.Id,
		},
		Pagination: &pbv1beta2.ResponsePagination{
			ContinuationToken: response.Pagination.ContinuationToken,
		},
	}
}
