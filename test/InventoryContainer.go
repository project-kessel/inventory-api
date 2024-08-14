package test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/ory/dockertest/v3"
	"github.com/ory/dockertest/v3/docker"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type LocalInventoryContainer struct {
	Name              string
	logger            log.Logger
	HTTPport          string
	gRPCport          string
	container         *dockertest.Resource
	pool              *dockertest.Pool
	network           string
	kccontainer       *dockertest.Resource
	dbcontainer       *dockertest.Resource
	migrationResource *dockertest.Resource
}

func CreateInventoryAPIContainer(logger log.Logger) (*LocalInventoryContainer, error) {
	pool, err := dockertest.NewPool("")
	if err != nil {
		log.Fatalf("Could not construct pool: %s", err)
		return nil, err
	}

	networkName := "inventory-api-net"
	network, err := createNetwork(networkName, pool)
	if err != nil {
		log.Fatalf("failed to create network: %v", err)
	}

	dbresource := createPostgresDB(pool, network)
	log.Infof("Postgresql db container: %s", dbresource.Container.Name)

	dbName := strings.Trim(dbresource.Container.Name, "/")

	srcPath := "../inventory-api-compose.yaml"

	// Copy file to a temporary directory
	tempFilePath, tempDir, err := copyFileToTempDir(srcPath)
	if err != nil {
		log.Fatalf("Error copying file: %s", err)
	}
	log.Info(tempFilePath)
	defer os.RemoveAll(tempDir)
	// Create application container
	dbhost := fmt.Sprintf("--storage.postgres.host=%s", dbName)
	migrationCmds := []string{"migrate", "--storage.database=postgres",
		dbhost,
		"--storage.postgres.password=secret",
		"--storage.postgres.user=admin",
		"--storage.postgres.dbname=invdb",
	}

	srcRealmPath := "../myrealm.json"

	// Copy file to a temporary directory
	tempRealmFilePath, tempRealmDir, err := copyFileToTempDir(srcRealmPath)
	if err != nil {
		log.Fatalf("Error copying file: %s", err)
	}
	log.Info(tempRealmFilePath)
	defer os.RemoveAll(tempRealmDir)

	kcresource, err := createKeycloakContainer(pool, network, tempRealmFilePath)
	if err != nil {
		log.Fatalf("Could not start keycloak resource: %s", err)
	}
	log.Info(kcresource.Container.Name)
	portMetric := kcresource.GetPort("9000/tcp")

	kcname := strings.Trim(kcresource.Container.Name, "/")
	authServerUrl := fmt.Sprintf("http://%s:%s/realms/redhat-external", kcname, "8084")

	// Wait for the container to be ready
	if err := pool.Retry(func() error {
		err = checkKeycloakHealth(fmt.Sprintf("http://localhost:%s", portMetric))
		log.Info("Waiting for Keycloak to be ready...")
		time.Sleep(20 * time.Second)
		return err
	}); err != nil {
		log.Fatalf("Could not connect to keycloak container: %s", err)
	}

	migrationResource := createInventoryContainer("migration", migrationCmds, tempDir, pool, dbName, network)
	log.Info(migrationResource.Container.Name)

	cmds := []string{
		"serve",
		"--storage.database=postgres",
		dbhost,
		"--storage.postgres.password=secret",
		"--storage.postgres.user=admin",
		"--storage.postgres.dbname=invdb",
		"--eventing.eventer=stdout",
		"--authz.impl=allow-all",
		"--server.grpc.address=0.0.0.0:9081",
		"--server.http.address=0.0.0.0:8081",
		fmt.Sprintf("--authn.oidc.authn-server-url=%s", authServerUrl),
		"--authn.oidc.skip-client-id-check=true",
		"--authn.oidc.skip-issuer-check=true",
	}

	inventoryContainer := createInventoryContainer("inventory-api", cmds, tempDir, pool, dbName, network)

	httpPort := inventoryContainer.GetPort("8081/tcp")
	grpcPort := inventoryContainer.GetPort("9081/tcp")

	return &LocalInventoryContainer{
		dbcontainer:       dbresource,
		container:         inventoryContainer,
		HTTPport:          httpPort,
		gRPCport:          grpcPort,
		network:           networkName,
		pool:              pool,
		kccontainer:       kcresource,
		migrationResource: migrationResource,
	}, nil
}

func createInventoryContainer(name string, cmds []string, tempDir string, pool *dockertest.Pool, databaseUrl string, network *docker.Network) *dockertest.Resource {
	targetArch := "amd64" // or "arm64", depending on your needs
	appresource, err := pool.BuildAndRunWithBuildOptions(&dockertest.BuildOptions{
		Dockerfile: "Dockerfile", // Path to your Dockerfile
		ContextDir: "../",        // Context directory for the Dockerfile
		Platform:   "linux/amd64",
		BuildArgs: []docker.BuildArg{
			{Name: "TARGETARCH", Value: targetArch},
		},
	}, &dockertest.RunOptions{
		Name:      name,
		Env:       []string{"INVENTORY_API_CONFIG=/inv-configs/inventory-api-compose.yaml"},
		Cmd:       cmds,
		NetworkID: network.ID,
		Mounts: []string{
			fmt.Sprintf("%s:/inv-configs", tempDir),
		},
	})
	if err != nil {
		log.Fatalf("Could not start resource: %s", err.Error())
	}
	err = pool.Client.Ping()
	if err != nil {
		log.Fatalf("Could not connect to Docker: %s", err)
	}
	return appresource
}

func createPostgresDB(pool *dockertest.Pool, network *docker.Network) *dockertest.Resource {
	dbresource, err := pool.RunWithOptions(&dockertest.RunOptions{
		Repository: "postgres",
		Tag:        "latest",
		Env: []string{
			"POSTGRES_PASSWORD=secret",
			"POSTGRES_USER=admin",
			"POSTGRES_DB=invdb",
			"listen_addresses = '*'",
		},
		NetworkID: network.ID,
	}, func(config *docker.HostConfig) {
		// set AutoRemove to true so that stopped container goes away by itself
		config.AutoRemove = true
		config.RestartPolicy = docker.RestartPolicy{Name: "no"}
	})
	if err != nil {
		log.Fatalf("Could not start dbresource: %s", err)
	}
	return dbresource
}

func createKeycloakContainer(pool *dockertest.Pool, network *docker.Network, tempRealmFilePath string) (*dockertest.Resource, error) {
	options := &dockertest.RunOptions{
		Repository: "quay.io/keycloak/keycloak",
		Tag:        "latest",
		Cmd: []string{
			"start-dev",
			"--http-port=8084",
			"--health-enabled=true",
			"--import-realm",
		},
		ExposedPorts: []string{"8084/tcp", "9000/tcp"},
		Env: []string{
			"KEYCLOAK_ADMIN=admin",
			"KEYCLOAK_ADMIN_PASSWORD=admin",
		},
		Mounts: []string{
			fmt.Sprintf("%s:/opt/keycloak/data/import/myrealm.json", tempRealmFilePath),
		},
		NetworkID: network.ID,
	}

	kcresource, err := pool.RunWithOptions(options, func(config *docker.HostConfig) {
		config.NetworkMode = "bridge"
		config.AutoRemove = true
		config.RestartPolicy = docker.RestartPolicy{Name: "no"}
	})
	return kcresource, err
}

func createNetwork(networkName string, pool *dockertest.Pool) (*docker.Network, error) {
	// Check if network exists
	network, err := findNetwork(networkName, pool)
	if err != nil {
		log.Fatalf("Could not list networks: %s", err)
	}
	if network == nil {
		network, err = pool.Client.CreateNetwork(docker.CreateNetworkOptions{
			Name:           networkName,
			Driver:         "bridge",
			CheckDuplicate: true,
		})
		if err != nil {
			log.Fatalf("Could not create network: %s", err)
		}
	}
	return network, err
}

func checkKeycloakHealth(baseURL string) error {
	resp, err := http.Get(fmt.Sprintf("%s/health", baseURL))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	return nil
}

func findNetwork(networkName string, pool *dockertest.Pool) (*docker.Network, error) {
	networks, err := pool.Client.ListNetworks()
	if err != nil {
		return nil, err
	}
	for _, net := range networks {
		if net.Name == networkName {
			return &net, nil
		}
	}
	return nil, nil
}

func copyFileToTempDir(srcPath string) (string, string, error) {
	// Create a temporary directory
	tempDir, err := os.MkdirTemp("", "inv-configs")
	if err != nil {
		return "", "", fmt.Errorf("could not create temp dir: %w", err)
	}
	//defer os.RemoveAll(tempDir) // Clean up the temp directory when done

	// Create the destination file path
	destPath := filepath.Join(tempDir, filepath.Base(srcPath))

	// Open the source file
	srcFile, err := os.Open(srcPath)
	if err != nil {
		return "", "", fmt.Errorf("could not open source file: %w", err)
	}
	defer srcFile.Close()

	// Create the destination file
	destFile, err := os.Create(destPath)
	if err != nil {
		return "", "", fmt.Errorf("could not create destination file: %w", err)
	}
	defer destFile.Close()

	// Copy the file contents
	if _, err := io.Copy(destFile, srcFile); err != nil {
		return "", "", fmt.Errorf("could not copy file: %w", err)
	}

	// Return the path to the copied file
	return destPath, tempDir, nil
}

type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token"`
}

func GetJWTToken(baseURL string) (*TokenResponse, error) {
	client := &http.Client{}
	data := url.Values{}
	data.Set("client_id", "svc-test")
	data.Set("client_secret", "h91qw8bPiDj9R6VSORsI5TYbceGU5PMH")
	data.Set("grant_type", "client_credentials")

	req, err := http.NewRequest("POST", fmt.Sprintf("%s/realms/redhat-external/protocol/openid-connect/token", baseURL), bytes.NewBufferString(data.Encode()))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var tokenResponse TokenResponse
	if err := json.Unmarshal(body, &tokenResponse); err != nil {
		return nil, err
	}

	return &tokenResponse, nil
}

func (l *LocalInventoryContainer) Close() {
	err := l.pool.Purge(l.container)
	if err != nil {
		log.NewHelper(l.logger).Error("Could not purge Kessel Inventory Container from test. Please delete manually.")
	}
	err = l.pool.Purge(l.kccontainer)
	if err != nil {
		log.NewHelper(l.logger).Error("Could not purge Keycloak Container from test. Please delete manually.")
	}
	err = l.pool.Purge(l.dbcontainer)
	if err != nil {
		log.NewHelper(l.logger).Error("Could not purge Postgres Container from test. Please delete manually.")
	}
	err = l.pool.Purge(l.migrationResource)
	if err != nil {
		log.NewHelper(l.logger).Error("Could not purge Inventory Migration Container from test. Please delete manually.")
	}
	if err := l.pool.Client.RemoveNetwork(l.network); err != nil {
		log.Fatalf("Could not remove network: %s", err)
	}
}
