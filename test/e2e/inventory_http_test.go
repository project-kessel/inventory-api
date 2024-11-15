package e2e

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/transport/http"
	v1 "github.com/project-kessel/inventory-api/api/kessel/inventory/v1"
	"github.com/project-kessel/inventory-api/api/kessel/inventory/v1beta1/relationships"
	"github.com/project-kessel/inventory-api/api/kessel/inventory/v1beta1/resources"
	"github.com/project-kessel/inventory-client-go/v1beta1"
	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/types/known/timestamppb"
	nethttp "net/http"
	"os"
	"strconv"
	"testing"
	"time"
)

var inventoryapi_http_url string
var tlsConfig *tls.Config
var insecure bool

func TestMain(m *testing.M) {
	inventoryapi_http_url = os.Getenv("INV_HTTP_URL")
	if inventoryapi_http_url == "" {
		err := fmt.Errorf("INV_HTTP_URL environment variable not set")
		log.Error(err)
		inventoryapi_http_url = "localhost:8081"
	}
	insecure = true
	insecureTLSstr := os.Getenv("INV_TLS_INSECURE")
	if insecureTLSstr != "" {
		var err error
		insecure, err = strconv.ParseBool(insecureTLSstr)
		if err != nil {
			log.Errorf("faild to parse bool INV_TLS_INSECURE %s", err)
		}
	}
	certFile := os.Getenv("INV_TLS_CERT_FILE")
	keyFile := os.Getenv("INV_TLS_KEY_FILE")
	caFile := os.Getenv("INV_TLS_CA_FILE")
	if certFile != "" && keyFile != "" && caFile != "" {
		// Load client cert
		cert, err := tls.LoadX509KeyPair(certFile, keyFile)
		if err != nil {
			log.Errorf("failed to load client certificate: %v", err)
		}
		// Load CA cert
		caCert, err := os.ReadFile(caFile)
		if err != nil {
			log.Errorf("failed to read CA certificate: %v", err)
		}
		caCertPool := x509.NewCertPool()
		if !caCertPool.AppendCertsFromPEM(caCert) {
			log.Errorf("failed to append CA certificate")
		}
		tlsConfig = &tls.Config{
			Certificates: []tls.Certificate{cert},
			RootCAs:      caCertPool,
		}
	} else {
		insecure = true
		log.Info("TLS environment variables not set")
	}
	result := m.Run()
	os.Exit(result)
}
func TestInventoryAPIHTTP_Livez(t *testing.T) {
	t.Parallel()
	httpClient, err := http.NewClient(
		context.Background(),
		http.WithEndpoint(inventoryapi_http_url),
	)
	if err != nil {
		t.Fatal("Failed to create HTTP client: ", err)
	}
	healthClient := v1.NewKesselInventoryHealthServiceHTTPClient(httpClient)
	resp, err := healthClient.GetLivez(context.Background(), &v1.GetLivezRequest{})
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	expectedStatus := "OK"
	expectedCode := uint32(200)
	assert.Equal(t, expectedStatus, resp.Status)
	assert.Equal(t, expectedCode, resp.Code)
}
func TestInventoryAPIHTTP_Readyz(t *testing.T) {
	t.Parallel()
	httpClient, err := http.NewClient(
		context.Background(),
		http.WithEndpoint(inventoryapi_http_url),
	)
	if err != nil {
		t.Fatal("Failed to create HTTP client: ", err)
	}
	healthClient := v1.NewKesselInventoryHealthServiceHTTPClient(httpClient)
	resp, err := healthClient.GetReadyz(context.Background(), &v1.GetReadyzRequest{})
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	expectedStatus := "RELATIONS-API"
	expectedCode := uint32(200)
	assert.Equal(t, expectedStatus, resp.Status)
	assert.Equal(t, expectedCode, resp.Code)
}
func TestInventoryAPIHTTP_Metrics(t *testing.T) {
	resp, err := nethttp.Get("http://" + inventoryapi_http_url + "/metrics")
	if err != nil {
		t.Fatal("Failed to send request: ", err)
	}
	defer resp.Body.Close()
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	expectedStatusCode := 200
	expectedStatusString := "200 OK"
	assert.Equal(t, expectedStatusCode, resp.StatusCode)
	assert.Equal(t, expectedStatusString, resp.Status)
}

func TestInventoryAPIHTTP_RHELHostLifecycle(t *testing.T) {
	t.Parallel()
	c := v1beta1.NewConfig(
		v1beta1.WithHTTPUrl(inventoryapi_http_url),
		v1beta1.WithTLSInsecure(insecure),
		v1beta1.WithHTTPTLSConfig(tlsConfig),
	)
	client, err := v1beta1.NewHttpClient(context.Background(), c)
	if err != nil {
		t.Error(err)
	}
	createRequest := resources.CreateRhelHostRequest{
		RhelHost: &resources.RhelHost{
			Metadata: &resources.Metadata{
				ResourceType: "rhel_host",
				WorkspaceId:  "workspace",
				OrgId:        "",
			},
			ReporterData: &resources.ReporterData{
				ReporterInstanceId: "user@example.com",
				ReporterType:       resources.ReporterData_ACM,
				ConsoleHref:        "www.example.com",
				ApiHref:            "www.example.com",
				LocalResourceId:    "0123",
				ReporterVersion:    "0.1",
			},
		},
	}
	opts := getCallOptions()
	_, err = client.RhelHostServiceClient.CreateRhelHost(context.Background(), &createRequest, opts...)
	assert.NoError(t, err, "Failed to create RhelHost")

	updateRequest := resources.UpdateRhelHostRequest{
		RhelHost: &resources.RhelHost{
			Metadata: &resources.Metadata{
				ResourceType: "rhel_host",
				WorkspaceId:  "workspace6",
				OrgId:        "",
			},
			ReporterData: &resources.ReporterData{
				ReporterInstanceId: "user@example.com",
				ReporterType:       resources.ReporterData_ACM,
				ConsoleHref:        "www.exampleConsole.com",
				ApiHref:            "www.exampleAPI.com",
				LocalResourceId:    "0123",
				ReporterVersion:    "0.1",
			},
		},
	}
	_, err = client.RhelHostServiceClient.UpdateRhelHost(context.Background(), &updateRequest, opts...)
	assert.NoError(t, err, "Failed to update RhelHost")

	deleteRequest := resources.DeleteRhelHostRequest{
		ReporterData: &resources.ReporterData{
			ReporterInstanceId: "user@example.com",
			ReporterType:       resources.ReporterData_ACM,
			ConsoleHref:        "www.exampleConsole.com",
			ApiHref:            "www.exampleAPI.com",
			LocalResourceId:    "0123",
			ReporterVersion:    "0.1",
		},
	}
	_, err = client.RhelHostServiceClient.DeleteRhelHost(context.Background(), &deleteRequest, opts...)
	assert.NoError(t, err, "Failed to delete RhelHost")
}

func TestInventoryAPIHTTP_K8SClusterLifecycle(t *testing.T) {
	t.Parallel()
	c := v1beta1.NewConfig(
		v1beta1.WithHTTPUrl(inventoryapi_http_url),
		v1beta1.WithTLSInsecure(insecure),
		v1beta1.WithHTTPTLSConfig(tlsConfig),
	)
	client, err := v1beta1.NewHttpClient(context.Background(), c)
	if err != nil {
		t.Error(err)
	}
	request := resources.CreateK8SClusterRequest{
		K8SCluster: &resources.K8SCluster{
			Metadata: &resources.Metadata{
				ResourceType: "k8s_cluster",
				WorkspaceId:  "workspace1",
				OrgId:        "",
			},
			ResourceData: &resources.K8SClusterDetail{
				ExternalClusterId: "01234",
				ClusterStatus:     resources.K8SClusterDetail_READY,
				KubeVersion:       "1.31",
				KubeVendor:        resources.K8SClusterDetail_OPENSHIFT,
				VendorVersion:     "4.16",
				CloudPlatform:     resources.K8SClusterDetail_AWS_UPI,
				Nodes: []*resources.K8SClusterDetailNodesInner{
					{
						Name:   "www.web.com",
						Cpu:    "7500m",
						Memory: "30973224Ki",
						Labels: []*resources.ResourceLabel{
							{
								Key:   "has_monster_gpu",
								Value: "no",
							},
						},
					},
				},
			},
			ReporterData: &resources.ReporterData{
				ReporterInstanceId: "user@example.com",
				ReporterType:       resources.ReporterData_ACM,
				ConsoleHref:        "www.example.com",
				ApiHref:            "www.example.com",
				LocalResourceId:    "01234",
				ReporterVersion:    "0.1",
			},
		},
	}
	opts := getCallOptions()
	_, err = client.K8sClusterService.CreateK8SCluster(context.Background(), &request, opts...)
	assert.NoError(t, err)

	updateRequest := resources.UpdateK8SClusterRequest{
		K8SCluster: &resources.K8SCluster{
			Metadata: &resources.Metadata{
				ResourceType: "k8s_cluster",
				WorkspaceId:  "workspace7",
				OrgId:        "",
			},
			ResourceData: &resources.K8SClusterDetail{
				ExternalClusterId: "01234",
				ClusterStatus:     resources.K8SClusterDetail_OFFLINE,
				KubeVersion:       "1.31",
				KubeVendor:        resources.K8SClusterDetail_OPENSHIFT,
				VendorVersion:     "4.16",
				CloudPlatform:     resources.K8SClusterDetail_AWS_UPI,
				Nodes: []*resources.K8SClusterDetailNodesInner{
					{
						Name:   "www.website.com",
						Cpu:    "7500m",
						Memory: "30973224Ki",
						Labels: []*resources.ResourceLabel{
							{
								Key:   "has_a_monster_gpu",
								Value: "yes",
							},
						},
					},
				},
			},
			ReporterData: &resources.ReporterData{
				ReporterInstanceId: "user@example.com",
				ReporterType:       resources.ReporterData_ACM,
				ConsoleHref:        "www.example.com",
				ApiHref:            "www.example.com",
				LocalResourceId:    "01234",
				ReporterVersion:    "0.1",
			},
		},
	}

	_, err = client.K8sClusterService.UpdateK8SCluster(context.Background(), &updateRequest, opts...)
	assert.NoError(t, err, "Failed to update K8sCluster")

	deleteRequest := resources.DeleteK8SClusterRequest{
		ReporterData: &resources.ReporterData{
			ReporterInstanceId: "user@example.com",
			ReporterType:       resources.ReporterData_ACM,
			ConsoleHref:        "www.example.com",
			ApiHref:            "www.example.com",
			LocalResourceId:    "01234",
			ReporterVersion:    "0.1",
		},
	}
	_, err = client.K8sClusterService.DeleteK8SCluster(context.Background(), &deleteRequest, opts...)
	assert.NoError(t, err, "Failed to delete K8sCluster")
}

func TestInventoryAPIHTTP_K8SPolicyLifecycle(t *testing.T) {
	t.Parallel()
	c := v1beta1.NewConfig(
		v1beta1.WithHTTPUrl(inventoryapi_http_url),
		v1beta1.WithTLSInsecure(insecure),
		v1beta1.WithHTTPTLSConfig(tlsConfig),
	)
	client, err := v1beta1.NewHttpClient(context.Background(), c)
	if err != nil {
		t.Error(err)
	}
	request := resources.CreateK8SPolicyRequest{
		K8SPolicy: &resources.K8SPolicy{
			Metadata: &resources.Metadata{
				ResourceType: "k8s_policy",
				WorkspaceId:  "workspace8",
				OrgId:        "",
			},
			ResourceData: &resources.K8SPolicyDetail{
				Disabled: false,
				Severity: resources.K8SPolicyDetail_HIGH,
			},
			ReporterData: &resources.ReporterData{
				ReporterInstanceId: "user@example.com",
				ReporterType:       resources.ReporterData_ACM,
				ConsoleHref:        "www.example.com",
				ApiHref:            "www.example.com",
				LocalResourceId:    "012345",
				ReporterVersion:    "0.1",
			},
		},
	}
	opts := getCallOptions()
	_, err = client.PolicyServiceClient.CreateK8SPolicy(context.Background(), &request, opts...)
	assert.NoError(t, err, "Failed to create K8sPolicy")

	updateRequest := resources.UpdateK8SPolicyRequest{
		K8SPolicy: &resources.K8SPolicy{
			Metadata: &resources.Metadata{
				ResourceType: "k8s_policy",
				WorkspaceId:  "workspace2",
				OrgId:        "",
			},
			ResourceData: &resources.K8SPolicyDetail{
				Disabled: true,
				Severity: resources.K8SPolicyDetail_LOW,
			},
			ReporterData: &resources.ReporterData{
				ReporterInstanceId: "user@example.com",
				ReporterType:       resources.ReporterData_ACM,
				ConsoleHref:        "www.example.com",
				ApiHref:            "www.example.com",
				LocalResourceId:    "012345",
				ReporterVersion:    "0.1",
			},
		},
	}

	_, err = client.PolicyServiceClient.UpdateK8SPolicy(context.Background(), &updateRequest, opts...)
	assert.NoError(t, err, "Failed to update K8sPolicy")

	deleteRequest := resources.DeleteK8SPolicyRequest{
		ReporterData: &resources.ReporterData{
			ReporterInstanceId: "user@example.com",
			ReporterType:       resources.ReporterData_ACM,
			ConsoleHref:        "www.example.com",
			ApiHref:            "www.example.com",
			LocalResourceId:    "012345",
			ReporterVersion:    "0.1",
		},
	}
	_, err = client.PolicyServiceClient.DeleteK8SPolicy(context.Background(), &deleteRequest, opts...)
	assert.NoError(t, err, "Failed to delete K8sPolicy")
}

func TestInventoryAPIHTTP_K8SPolicy_is_propagated_to_K8sClusterLifecycle(t *testing.T) {
	t.Parallel()
	c := v1beta1.NewConfig(
		v1beta1.WithHTTPUrl(inventoryapi_http_url),
		v1beta1.WithTLSInsecure(insecure),
		v1beta1.WithHTTPTLSConfig(tlsConfig),
	)
	client, err := v1beta1.NewHttpClient(context.Background(), c)
	if err != nil {
		t.Error(err)
	}
	request := resources.CreateK8SPolicyRequest{
		K8SPolicy: &resources.K8SPolicy{
			Metadata: &resources.Metadata{
				ResourceType: "k8s_policy",
				WorkspaceId:  "workspace2",
				OrgId:        "",
			},
			ResourceData: &resources.K8SPolicyDetail{
				Disabled: false,
				Severity: resources.K8SPolicyDetail_HIGH,
			},
			ReporterData: &resources.ReporterData{
				ReporterInstanceId: "user@example.com",
				ReporterType:       resources.ReporterData_ACM,
				ConsoleHref:        "www.example.com",
				ApiHref:            "www.example.com",
				LocalResourceId:    "789",
				ReporterVersion:    "0.1",
			},
		},
	}
	opts := getCallOptions()
	_, err = client.PolicyServiceClient.CreateK8SPolicy(context.Background(), &request, opts...)
	assert.NoError(t, err, "Failed to create K8sPolicy")

	request1 := resources.CreateK8SClusterRequest{
		K8SCluster: &resources.K8SCluster{
			Metadata: &resources.Metadata{
				ResourceType: "k8s_cluster",
				WorkspaceId:  "workspace2",
				OrgId:        "",
			},
			ResourceData: &resources.K8SClusterDetail{
				ExternalClusterId: "01234",
				ClusterStatus:     resources.K8SClusterDetail_READY,
				KubeVersion:       "1.31",
				KubeVendor:        resources.K8SClusterDetail_OPENSHIFT,
				VendorVersion:     "4.16",
				CloudPlatform:     resources.K8SClusterDetail_AWS_UPI,
				Nodes: []*resources.K8SClusterDetailNodesInner{
					{
						Name:   "www.web.com",
						Cpu:    "7500m",
						Memory: "30973224Ki",
						Labels: []*resources.ResourceLabel{
							{
								Key:   "has_a_monster_gpu",
								Value: "no",
							},
						},
					},
				},
			},
			ReporterData: &resources.ReporterData{
				ReporterInstanceId: "user@example.com",
				ReporterType:       resources.ReporterData_ACM,
				ConsoleHref:        "www.example.com",
				ApiHref:            "www.example.com",
				LocalResourceId:    "987",
				ReporterVersion:    "0.1",
			},
		},
	}
	_, err = client.K8sClusterService.CreateK8SCluster(context.Background(), &request1, opts...)
	assert.NoError(t, err, "Failed to create K8sCluster")

	requestRelationship := relationships.CreateK8SPolicyIsPropagatedToK8SClusterRequest{
		K8SpolicyIspropagatedtoK8Scluster: &relationships.K8SPolicyIsPropagatedToK8SCluster{
			Metadata: &relationships.Metadata{
				RelationshipType: "k8spolicy_ispropagatedto_k8scluster",
				OrgId:            "",
				CreatedAt:        timestamppb.New(time.Now()),
				UpdatedAt:        timestamppb.New(time.Now()),
			},
			ReporterData: &relationships.ReporterData{
				ReporterType:           relationships.ReporterData_ACM,
				ReporterVersion:        "0.1",
				SubjectLocalResourceId: "789", // LocalResourceID of K8SPolicy
				ObjectLocalResourceId:  "987", // LocalResourceID of K8SCluster
			},
			RelationshipData: &relationships.K8SPolicyIsPropagatedToK8SClusterDetail{
				Status: relationships.K8SPolicyIsPropagatedToK8SClusterDetail_NO_VIOLATIONS,
			},
		},
	}

	_, err = client.K8SPolicyIsPropagatedToK8SClusterServiceHTTPClient.CreateK8SPolicyIsPropagatedToK8SCluster(context.Background(), &requestRelationship, opts...)
	assert.NoError(t, err, "Failed to create relationship between K8sPolicy and K8sCluster")

	updateRequest := relationships.UpdateK8SPolicyIsPropagatedToK8SClusterRequest{
		K8SpolicyIspropagatedtoK8Scluster: &relationships.K8SPolicyIsPropagatedToK8SCluster{
			Metadata: &relationships.Metadata{
				RelationshipType: "k8spolicy_ispropagatedto_k8scluster",
				OrgId:            "",
				CreatedAt:        timestamppb.New(time.Now()),
				UpdatedAt:        timestamppb.New(time.Now()),
			},
			ReporterData: &relationships.ReporterData{
				ReporterType:           relationships.ReporterData_ACM,
				ReporterVersion:        "0.1",
				SubjectLocalResourceId: "789", // LocalResourceID of K8SPolicy
				ObjectLocalResourceId:  "987", // LocalResourceID of K8SCluster
			},
			RelationshipData: &relationships.K8SPolicyIsPropagatedToK8SClusterDetail{
				Status: relationships.K8SPolicyIsPropagatedToK8SClusterDetail_VIOLATIONS,
			},
		},
	}

	_, err = client.K8SPolicyIsPropagatedToK8SClusterServiceHTTPClient.UpdateK8SPolicyIsPropagatedToK8SCluster(context.Background(), &updateRequest, opts...)
	assert.NoError(t, err, "Failed to update relationship between K8sPolicy and K8sCluster")

	deleteRequest := relationships.DeleteK8SPolicyIsPropagatedToK8SClusterRequest{
		ReporterData: &relationships.ReporterData{
			ReporterType:           relationships.ReporterData_ACM,
			ReporterVersion:        "0.1",
			SubjectLocalResourceId: "789", // LocalResourceID of K8SPolicy
			ObjectLocalResourceId:  "987", // LocalResourceID of K8SCluster
		},
	}

	_, err = client.K8SPolicyIsPropagatedToK8SClusterServiceHTTPClient.DeleteK8SPolicyIsPropagatedToK8SCluster(context.Background(), &deleteRequest, opts...)
	assert.NoError(t, err, "Failed to delete relationship between K8sPolicy and K8sCluster")
}

func getCallOptions() []http.CallOption {
	var opts []http.CallOption
	header := nethttp.Header{}
	header.Set("Authorization", fmt.Sprintf("Bearer %s", "1234"))
	opts = append(opts, http.Header(&header))
	return opts
}
