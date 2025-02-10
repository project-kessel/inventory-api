package middleware

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/go-kratos/kratos/v2/middleware"
	"github.com/go-kratos/kratos/v2/transport"
	"github.com/go-kratos/kratos/v2/transport/http"
	pb "github.com/project-kessel/inventory-api/api/kessel/inventory/v1beta1/resources"
	"github.com/project-kessel/inventory-api/internal/biz/model"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/structpb"
	"io"
)

type K8SClusterPayload struct {
	K8SCluster struct {
		Metadata struct {
			ResourceType string `json:"resource_type"`
			WorkspaceID  string `json:"workspace_id"`
		} `json:"metadata"`
		ReporterData struct {
			ReporterType    string `json:"reporter_type"`
			ReporterVersion string `json:"reporter_version"`
			LocalResourceID string `json:"local_resource_id"`
			APIHref         string `json:"api_href"`
			ConsoleHref     string `json:"console_href"`
		} `json:"reporter_data"`
		ResourceData struct {
			ExternalClusterID string  `json:"external_cluster_id"`
			ClusterStatus     string  `json:"cluster_status"`
			ClusterReason     *string `json:"cluster_reason"`
			KubeVersion       string  `json:"kube_version"`
			KubeVendor        string  `json:"kube_vendor"`
			VendorVersion     string  `json:"vendor_version"`
			CloudPlatform     string  `json:"cloud_platform"`
			Nodes             []struct {
				Name   string `json:"name"`
				CPU    string `json:"cpu"`
				Memory string `json:"memory"`
				Labels []struct {
					Key   string `json:"key"`
					Value string `json:"value"`
				} `json:"labels"`
			} `json:"nodes"`
		} `json:"resource_data"`
	} `json:"k8s_cluster"`
}

type K8SPolicyPayload struct {
	K8SPolicy struct {
		Metadata struct {
			ResourceType string `json:"resource_type"`
			WorkspaceID  string `json:"workspace_id"`
		} `json:"metadata"`
		ReporterData struct {
			ReporterType    string `json:"reporter_type"`
			ReporterVersion string `json:"reporter_version"`
			LocalResourceID string `json:"local_resource_id"`
			APIHref         string `json:"api_href"`
			ConsoleHref     string `json:"console_href"`
		} `json:"reporter_data"`
		ResourceData struct {
			Disabled bool   `json:"disabled"`
			Severity string `json:"severity"`
		} `json:"resource_data"`
	} `json:"k8s_policy"`
}

type IntegrationPayload struct {
	NotificationsIntegration struct {
		Metadata struct {
			ResourceType string `json:"resource_type"`
			WorkspaceID  string `json:"workspace_id"`
		} `json:"metadata"`
		ReporterData struct {
			ReporterType    string `json:"reporter_type"`
			ReporterVersion string `json:"reporter_version"`
			LocalResourceID string `json:"local_resource_id"`
			APIHref         string `json:"api_href"`
			ConsoleHref     string `json:"console_href"`
		} `json:"reporter_data"`
	} `json:"integration"`
}

type RhelHostPayload struct {
	RhelHost struct {
		Metadata struct {
			ResourceType string `json:"resource_type"`
			WorkspaceID  string `json:"workspace_id"`
		} `json:"metadata"`
		ReporterData struct {
			ReporterType    string `json:"reporter_type"`
			ReporterVersion string `json:"reporter_version"`
			LocalResourceID string `json:"local_resource_id"`
			APIHref         string `json:"api_href"`
			ConsoleHref     string `json:"console_href"`
		} `json:"reporter_data"`
	} `json:"rhel_host"`
}

func UnaryServerInterceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	type ResourceData struct {
		Metadata     model.JsonObject `json:"metadata"`
		ReporterData model.JsonObject `json:"reporter_data"`
		ResourceData model.JsonObject `json:"resource_data,omitempty"`
	}
	if payload, ok := req.(*pb.UpdateK8SClusterRequest); ok {
		// For debugging.
		fmt.Println("Received payload:", payload)

		// Create a generic resource by transforming fields from the payload.
		resource := &ResourceData{
			Metadata: model.JsonObject{
				"resource_type": payload.K8SCluster.Metadata.ResourceType,
				"workspace_id":  payload.K8SCluster.Metadata.WorkspaceId,
			},
			ReporterData: model.JsonObject{
				"reporter_type":     payload.K8SCluster.ReporterData.ReporterType,
				"reporter_version":  payload.K8SCluster.ReporterData.ReporterVersion,
				"local_resource_id": payload.K8SCluster.ReporterData.LocalResourceId,
				"api_href":          payload.K8SCluster.ReporterData.ApiHref,
				"console_href":      payload.K8SCluster.ReporterData.ConsoleHref,
			},
			ResourceData: model.JsonObject{
				"external_cluster_id": payload.K8SCluster.ResourceData.ExternalClusterId,
			},
		}

		// Pass the transformed resource down the chain.
		return handler(ctx, resource)
	}

	// If the request is not of type *Payload, pass it down the chain as-is.
	return handler(ctx, req)
}

func TransformMiddleware() middleware.Middleware {
	return func(handler middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req interface{}) (interface{}, error) {
			if tr, ok := transport.FromServerContext(ctx); ok {
				if ht, ok := tr.(*http.Transport); ok {
					requestURI := ht.Request().RequestURI
					body, err := io.ReadAll(ht.Request().Body)
					if err != nil {
						return nil, err
					}
					var resource interface{}
					switch requestURI {
					case "/api/inventory/v1beta1/resources/k8s-clusters":
						var k8spayload K8SClusterPayload
						if err := json.Unmarshal(body, &k8spayload); err != nil {
							return nil, err
						}
						resource = createK8SClusterResource(k8spayload)

					case "/api/inventory/v1beta1/resources/k8s-policies":
						var k8spolicyPayload K8SPolicyPayload
						if err := json.Unmarshal(body, &k8spolicyPayload); err != nil {
							return nil, err
						}
						resource = createK8SPolicyResource(k8spolicyPayload)

					case "/api/inventory/v1beta1/resources/rhel-hosts":
						var rhelHostpayload RhelHostPayload
						if err := json.Unmarshal(body, &rhelHostpayload); err != nil {
							return nil, err
						}
						resource = createRhelHostResource(rhelHostpayload)

					case "/api/inventory/v1beta1/resources/notifications-integrations":
						var integrationPayload IntegrationPayload
						if err := json.Unmarshal(body, &integrationPayload); err != nil {
							return nil, err
						}
						resource = createNotificationIntegrationResource(integrationPayload)

					case "/api/inventory/v1beta1/resource-relationships/k8s-policy_is-propagated-to_k8s-cluster":
						return handler(ctx, body)

					default:
						return nil, fmt.Errorf("invalid endpoint: %s", requestURI)
					}

					if resource != nil {
						return handler(ctx, resource)
					}
				}
			}
			return handler(ctx, req)
		}
	}
}

func createRhelHostResource(hostpayload RhelHostPayload) interface{} {
	reporterData, _ := createReporterData(hostpayload.RhelHost.ReporterData)

	return &pb.UpdateResourceRequest{
		Resource: &pb.Resource{
			Metadata: &pb.Metadata{
				ResourceType: hostpayload.RhelHost.Metadata.ResourceType,
				WorkspaceId:  hostpayload.RhelHost.Metadata.WorkspaceID,
			},
			ReporterData: reporterData,
		},
	}
}

func createReporterData(reporter struct {
	ReporterType    string `json:"reporter_type"`
	ReporterVersion string `json:"reporter_version"`
	LocalResourceID string `json:"local_resource_id"`
	APIHref         string `json:"api_href"`
	ConsoleHref     string `json:"console_href"`
}) (*structpb.Struct, error) {
	return structpb.NewStruct(map[string]interface{}{
		"reporter_type":     reporter.ReporterType,
		"reporter_version":  reporter.ReporterVersion,
		"local_resource_id": reporter.LocalResourceID,
		"api_href":          reporter.APIHref,
		"console_href":      reporter.ConsoleHref,
	})
}

func createK8SClusterResource(payload K8SClusterPayload) *pb.UpdateResourceRequest {
	reporterData, _ := createReporterData(payload.K8SCluster.ReporterData)

	resourceData, _ := structpb.NewStruct(map[string]interface{}{
		"external_cluster_id": payload.K8SCluster.ResourceData.ExternalClusterID,
		"cluster_status":      payload.K8SCluster.ResourceData.ClusterStatus,
		"cluster_reason":      payload.K8SCluster.ResourceData.ClusterReason,
		"kube_version":        payload.K8SCluster.ResourceData.KubeVersion,
		"kube_vendor":         payload.K8SCluster.ResourceData.KubeVendor,
		"vendor_version":      payload.K8SCluster.ResourceData.VendorVersion,
		"cloud_platform":      payload.K8SCluster.ResourceData.CloudPlatform,
	})

	return &pb.UpdateResourceRequest{
		Resource: &pb.Resource{
			Metadata: &pb.Metadata{
				ResourceType: payload.K8SCluster.Metadata.ResourceType,
				WorkspaceId:  payload.K8SCluster.Metadata.WorkspaceID,
			},
			ReporterData: reporterData,
			ResourceData: resourceData,
		},
	}
}

func createK8SPolicyResource(payload K8SPolicyPayload) *pb.CreateResourceRequest {
	reporterData, _ := createReporterData(payload.K8SPolicy.ReporterData)

	resourceData, _ := structpb.NewStruct(map[string]interface{}{
		"severity": payload.K8SPolicy.ResourceData.Severity,
		"disabled": payload.K8SPolicy.ResourceData.Disabled,
	})

	return &pb.CreateResourceRequest{
		Resource: &pb.Resource{
			Metadata: &pb.Metadata{
				ResourceType: payload.K8SPolicy.Metadata.ResourceType,
				WorkspaceId:  payload.K8SPolicy.Metadata.WorkspaceID,
			},
			ReporterData: reporterData,
			ResourceData: resourceData,
		},
	}
}

func createNotificationIntegrationResource(payload IntegrationPayload) *pb.UpdateResourceRequest {
	reporterData, _ := createReporterData(payload.NotificationsIntegration.ReporterData)

	return &pb.UpdateResourceRequest{
		Resource: &pb.Resource{
			Metadata: &pb.Metadata{
				ResourceType: payload.NotificationsIntegration.Metadata.ResourceType,
				WorkspaceId:  payload.NotificationsIntegration.Metadata.WorkspaceID,
			},
			ReporterData: reporterData,
		},
	}
}
