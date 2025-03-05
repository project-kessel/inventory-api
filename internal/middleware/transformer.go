package middleware

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/go-kratos/kratos/v2/middleware"
	"github.com/go-kratos/kratos/v2/transport"
	"github.com/go-kratos/kratos/v2/transport/http"
	"github.com/project-kessel/inventory-api/api/kessel/inventory/v1beta2"
	pbv2 "github.com/project-kessel/inventory-api/api/kessel/inventory/v1beta2"
	"google.golang.org/protobuf/types/known/structpb"
	"io"
	"strings"
)

type ReporterDataPayload struct {
	ReporterType       string                 `json:"reporter_type"`
	ReporterInstanceID string                 `json:"reporter_instance_id"`
	ReporterVersion    string                 `json:"reporter_version"`
	LocalResourceID    string                 `json:"local_resource_id"`
	APIHref            string                 `json:"api_href"`
	ConsoleHref        string                 `json:"console_href"`
	ResourceData       map[string]interface{} `json:"resource_data"`
}

type ResourcePayload struct {
	Resource struct {
		ResourceType       string                 `json:"resource_type"`
		ReporterData       ReporterDataPayload    `json:"reporter_data"`
		CommonResourceData map[string]interface{} `json:"common_resource_data"`
	} `json:"resource"`
}

func TransformMiddleware() middleware.Middleware {
	return func(handler middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req interface{}) (interface{}, error) {
			if tr, ok := transport.FromServerContext(ctx); ok {
				if ht, ok := tr.(*http.Transport); ok {
					requestURI := ht.Request().RequestURI
					if strings.Contains(requestURI, "v1beta2") {
						method := ht.Request().Method
						body, err := io.ReadAll(ht.Request().Body)
						if err != nil {
							return nil, err
						}

						var payload ResourcePayload
						if err := json.Unmarshal(body, &payload); err != nil {
							return nil, fmt.Errorf("failed to parse JSON payload: %w", err)
						}

						var resource interface{}
						if method == "DELETE" {
							resource = deleteResourceRequest(payload)
						} else {
							resource, err = createResourceRequest(payload)
							if err != nil {
								return nil, fmt.Errorf("failed to create resource request: %w", err)
							}
						}
						if resource != nil {
							return handler(ctx, resource)
						}
					}
				}
			}
			return handler(ctx, req)
		}
	}
}

func createReporterData(reporter ReporterDataPayload) (*structpb.Struct, error) {
	resMap := map[string]interface{}{
		"reporter_type":        reporter.ReporterType,
		"reporter_instance_id": reporter.ReporterInstanceID,
		"reporter_version":     reporter.ReporterVersion,
		"local_resource_id":    reporter.LocalResourceID,
		"api_href":             reporter.APIHref,
		"console_href":         reporter.ConsoleHref,
		"resource_data":        reporter.ResourceData,
	}
	resStruct, err := structpb.NewStruct(resMap)
	if err != nil {
		return nil, fmt.Errorf("failed to create reporter_data struct: %w", err)
	}
	return resStruct, nil
}

func createResourceRequest(payload ResourcePayload) (*pbv2.ReportResourceRequest, error) {
	// Debugging: Print payload before processing
	payloadJSON, _ := json.MarshalIndent(payload, "", "  ")
	fmt.Printf("DEBUG: Incoming ResourcePayload:\n%s\n", payloadJSON)

	// Convert reporter_data to structured data (map to Struct)
	reporterData, err := createReporterData(payload.Resource.ReporterData)
	if err != nil {
		return nil, fmt.Errorf("failed to create reporter data: %w", err)
	}

	// Convert common_resource_data to structured data
	var commonResourceData *structpb.Struct
	if payload.Resource.CommonResourceData != nil {
		commonResourceData, err = structpb.NewStruct(payload.Resource.CommonResourceData)
		if err != nil {
			return nil, fmt.Errorf("failed to create common_resource_data struct: %w", err)
		}
	} else {
		fmt.Println("DEBUG: common_resource_data is empty or missing")
	}

	return &pbv2.ReportResourceRequest{
		Resource: &pbv2.Resource{
			ResourceType: payload.Resource.ResourceType,
			ReporterData: &v1beta2.ReporterData{
				ReporterType:       payload.Resource.ReporterData.ReporterType,
				ReporterInstanceId: payload.Resource.ReporterData.ReporterInstanceID,
				ReporterVersion:    payload.Resource.ReporterData.ReporterVersion,
				LocalResourceId:    payload.Resource.ReporterData.LocalResourceID,
				ApiHref:            payload.Resource.ReporterData.APIHref,
				ConsoleHref:        payload.Resource.ReporterData.ConsoleHref,
				ResourceData:       reporterData,
			},
			CommonResourceData: commonResourceData,
		},
	}, nil
}

func deleteResourceRequest(payload ResourcePayload) *pbv2.DeleteResourceRequest {
	return &pbv2.DeleteResourceRequest{
		LocalResourceId: payload.Resource.ReporterData.LocalResourceID,
		ReporterType:    payload.Resource.ReporterData.ReporterType,
	}
}
