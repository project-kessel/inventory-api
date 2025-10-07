package e2e

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	nethttp "net/http"
	"os"
	"strconv"
	"testing"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/transport/http"
	v1 "github.com/project-kessel/inventory-api/api/kessel/inventory/v1"
	"github.com/stretchr/testify/assert"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var inventoryapi_http_url string
var tlsConfig *tls.Config
var insecure bool
var db *gorm.DB

// v1beta2
var inventoryapi_grpc_url string

func TestMain(m *testing.M) {
	var err error

	inventoryapi_http_url = os.Getenv("INV_HTTP_URL")
	if inventoryapi_http_url == "" {
		err := fmt.Errorf("INV_HTTP_URL environment variable not set")
		log.Error(err)
		inventoryapi_http_url = "localhost:8081"
	}
	inventoryapi_grpc_url = os.Getenv("INV_GRPC_URL")
	if inventoryapi_grpc_url == "" {
		err := fmt.Errorf("INV_GRPC_URL environment variable not set")
		log.Error(err)
		inventoryapi_grpc_url = "localhost:9081"
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

	dbUser := os.Getenv("POSTGRES_USER")
	if dbUser == "" {
		err := fmt.Errorf("POSTGRES_USER environment variable not set")
		log.Error(err)
	}
	dbPassword := os.Getenv("POSTGRES_PASSWORD")
	if dbPassword == "" {
		err := fmt.Errorf("POSTGRES_PASSWORD environment variable not set")
		log.Error(err)
	}
	dbHost := os.Getenv("POSTGRES_HOST")
	if dbHost == "" {
		err := fmt.Errorf("POSTGRES_HOST environment variable not set")
		log.Error(err)
	}
	dbPort := os.Getenv("POSTGRES_PORT")
	if dbPort == "" {
		err := fmt.Errorf("POSTGRES_PORT environment variable not set")
		log.Error(err)
	}
	dbName := os.Getenv("POSTGRES_DB")
	if dbName == "" {
		err := fmt.Errorf("POSTGRES_DB environment variable not set")
		log.Error(err)
	}

	dsn := "host=" + dbHost + " user=" + dbUser + " password=" + dbPassword + " port=" + dbPort + " dbname=" + dbName
	db, err = gorm.Open(postgres.Open(dsn), &gorm.Config{TranslateError: true})
	if err != nil {
		log.Errorf("failed to connect to database: %v", err)
	} else {
		// Clean up test data before running tests
		cleanupTestData()
	}

	result := m.Run()
	os.Exit(result)
}

func cleanupTestData() {
	if db != nil {
		// Clean up test data to avoid conflicts
		db.Exec("TRUNCATE TABLE reporter_representations, common_representations, reporter_resources, resource, outbox_events CASCADE")
	}
}

func TestInventoryAPIHTTP_Livez(t *testing.T) {
	enableShortMode(t)
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
	enableShortMode(t)
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
	expectedStatus := "STORAGE postgres and RELATIONS-API"
	expectedCode := uint32(200)
	assert.Equal(t, expectedStatus, resp.Status)
	assert.Equal(t, expectedCode, resp.Code)
}
func TestInventoryAPIHTTP_Metrics(t *testing.T) {
	enableShortMode(t)
	resp, err := nethttp.Get("http://" + inventoryapi_http_url + "/metrics")
	if err != nil {
		t.Fatal("Failed to send request: ", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			fmt.Printf("failed to close consumer: %v", err)
		}
	}()
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	expectedStatusCode := 200
	expectedStatusString := "200 OK"
	assert.Equal(t, expectedStatusCode, resp.StatusCode)
	assert.Equal(t, expectedStatusString, resp.Status)
}
