package e2e

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/transport/http"
	v1 "github.com/project-kessel/inventory-api/api/kessel/inventory/v1"
	"github.com/project-kessel/inventory-api/api/kessel/inventory/v1beta1/resources"
	"github.com/project-kessel/inventory-client-go/v1beta1"
	"github.com/stretchr/testify/assert"
	nethttp "net/http"
	"os"
	"strconv"
	"testing"
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

	expectedStatus := "Storage type postgres"
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

func TestInventoryAPIHTTP_CreateRHELHost(t *testing.T) {
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
	request := resources.CreateRhelHostRequest{RhelHost: &resources.RhelHost{
		Metadata: &resources.Metadata{
			ResourceType: "rhel-host",
			WorkspaceId:  "workspace1",
			OrgId:        "",
		},
		ReporterData: &resources.ReporterData{
			ReporterInstanceId: "user@example.com",
			ReporterType:       resources.ReporterData_OCM,
			ConsoleHref:        "www.example.com",
			ApiHref:            "www.example.com",
			LocalResourceId:    "1",
			ReporterVersion:    "0.1",
		},
	}}
	opts := getCallOptions()
	_, err = client.RhelHostServiceClient.CreateRhelHost(context.Background(), &request, opts...)
	assert.NoError(t, err)

}

func TestInventoryAPIHTTP_K8SCluster_CreateK8SCluster(t *testing.T) {
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
				ResourceType: "k8s-cluster",
				WorkspaceId:  "",
				OrgId:        "",
			},
			ResourceData: &resources.K8SClusterDetail{
				ExternalClusterId: "1234",
				ClusterStatus:     resources.K8SClusterDetail_READY,
				KubeVersion:       "1.31",
				KubeVendor:        resources.K8SClusterDetail_OPENSHIFT,
				VendorVersion:     "4.16",
				CloudPlatform:     resources.K8SClusterDetail_AWS_UPI,
				Nodes: []*resources.K8SClusterDetailNodesInner{
					{
						Name:   "www.example.com",
						Cpu:    "7500m",
						Memory: "30973224Ki",
						Labels: []*resources.ResourceLabel{
							{
								Key:   "has_monster_gpu",
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
				LocalResourceId:    "1",
				ReporterVersion:    "0.1",
			},
		},
	}
	opts := getCallOptions()
	_, err = client.K8sClusterService.CreateK8SCluster(context.Background(), &request, opts...)
	assert.NoError(t, err)
}

func TestInventoryAPIHTTP_K8SPolicy_CreateK8SPolicy(t *testing.T) {
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
				ResourceType: "k8s-policy",
				WorkspaceId:  "default",
				OrgId:        "",
			},
			ResourceData: &resources.K8SPolicyDetail{
				Disabled: true,
				Severity: resources.K8SPolicyDetail_MEDIUM,
			},
			ReporterData: &resources.ReporterData{
				ReporterInstanceId: "user@example.com",
				ReporterType:       resources.ReporterData_ACM,
				ConsoleHref:        "www.example.com",
				ApiHref:            "www.example.com",
				LocalResourceId:    "1",
				ReporterVersion:    "0.1",
			},
		},
	}
	opts := getCallOptions()
	_, err = client.PolicyServiceClient.CreateK8SPolicy(context.Background(), &request, opts...)
	assert.NoError(t, err)

}

func getCallOptions() []http.CallOption {
	var opts []http.CallOption
	header := nethttp.Header{}
	header.Set("Authorization", fmt.Sprintf("Bearer %s", "1234"))
	opts = append(opts, http.Header(&header))
	return opts
}
