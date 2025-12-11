package spicedb

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/ory/dockertest/v3"
	"github.com/ory/dockertest/v3/docker"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health/grpc_health_v1"
)

const (
	// SpicedbImage is the image used for containerized spiceDB in tests
	SpicedbImage = "authzed/spicedb"
	// SpicedbVersion is the image version used for containerized spiceDB in tests
	SpicedbVersion = "v1.37.0"
	// SpicedbSchemaBootstrapFile specifies the test schema file
	SpicedbSchemaBootstrapFile = "testdata/schema.zed"
	// FullyConsistent specifies the consistency mode used for read API calls
	FullyConsistent = false
)

// LocalSpiceDbContainer manages a containerized SpiceDB instance for testing
type LocalSpiceDbContainer struct {
	logger         log.Logger
	port           string
	container      *dockertest.Resource
	pool           *dockertest.Pool
	name           string
	schemaLocation string
}

// ContainerOptions for creating a SpiceDB test container
type ContainerOptions struct {
	Logger  log.Logger
	Network *docker.Network
}

// CreateContainer creates a new SpiceDbContainer using dockertest
func CreateContainer(opts *ContainerOptions) (*LocalSpiceDbContainer, error) {
	pool, err := dockertest.NewPool("") // Empty string uses default docker env
	if err != nil {
		return nil, fmt.Errorf("could not connect to docker: %w", err)
	}

	pool.MaxWait = 3 * time.Minute

	var (
		_, b, _, _ = runtime.Caller(0)
		basepath   = filepath.Dir(b)
	)

	cmd := []string{"serve-testing", "--skip-release-check=true"}

	runopt := &dockertest.RunOptions{
		Repository:   SpicedbImage,
		Tag:          SpicedbVersion,
		Cmd:          cmd,
		ExposedPorts: []string{"50051/tcp", "50052/tcp"},
	}
	if opts.Network != nil {
		runopt.NetworkID = opts.Network.ID
	}
	resource, err := pool.RunWithOptions(runopt)

	if err != nil {
		return nil, fmt.Errorf("could not start spicedb resource: %w", err)
	}

	port := resource.GetPort("50051/tcp")

	// Give the service time to boot
	cErr := pool.Retry(func() error {
		log.NewHelper(opts.Logger).Info("Attempting to connect to spicedb...")

		conn, err := grpc.NewClient(
			fmt.Sprintf("localhost:%s", port),
			grpc.WithTransportCredentials(insecure.NewCredentials()),
		)
		if err != nil {
			return fmt.Errorf("error connecting to spiceDB: %v", err.Error())
		}

		client := grpc_health_v1.NewHealthClient(conn)
		_, err = client.Check(context.Background(), &grpc_health_v1.HealthCheckRequest{})
		return err
	})

	if cErr != nil {
		return nil, cErr
	}

	return &LocalSpiceDbContainer{
		name:           resource.Container.Name,
		logger:         opts.Logger,
		port:           port,
		container:      resource,
		pool:           pool,
		schemaLocation: path.Join(basepath, SpicedbSchemaBootstrapFile),
	}, nil
}

// Port returns the port the container is listening on
func (l *LocalSpiceDbContainer) Port() string {
	return l.port
}

// Name returns the container name
func (l *LocalSpiceDbContainer) Name() string {
	return l.name
}

// NewToken returns a new token used for the container so a new store is created in serve-testing
func (l *LocalSpiceDbContainer) NewToken() (string, error) {
	buf := make([]byte, 20)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}

	return base64.StdEncoding.EncodeToString(buf), nil
}

// WaitForQuantizationInterval needed to avoid read-before-write when not using fully consistent mode
func (l *LocalSpiceDbContainer) WaitForQuantizationInterval() {
	if !FullyConsistent {
		time.Sleep(10 * time.Millisecond)
	}
}

// CreateSpiceDbRepository creates a new SpiceDbRepository connected to the test container
func (l *LocalSpiceDbContainer) CreateSpiceDbRepository() (*SpiceDbRepository, error) {
	randomKey, err := l.NewToken()
	if err != nil {
		return nil, err
	}

	// Create temp directory and file for the token
	tmpDir, err := os.MkdirTemp("", "inventory-api-spicedb-test")
	if err != nil {
		return nil, err
	}
	tmpFile, err := os.CreateTemp(tmpDir, "spicedbpreshared")
	if err != nil {
		return nil, err
	}
	err = os.WriteFile(tmpFile.Name(), []byte(randomKey), 0666)
	if err != nil {
		return nil, err
	}

	defer func() {
		if err := os.RemoveAll(tmpDir); err != nil {
			log.NewHelper(l.logger).Errorf("error removing temporary directory: %w", err)
		}
	}()

	// Create config for this test instance
	config := &Config{
		Endpoint:        "localhost:" + l.port,
		Token:           "", // Will use TokenFile instead
		TokenFile:       tmpFile.Name(),
		SchemaFile:      l.schemaLocation,
		UseTLS:          false,
		FullyConsistent: FullyConsistent,
	}

	repo, err := NewSpiceDbRepository(config, log.NewHelper(l.logger))
	if err != nil {
		return nil, err
	}

	return repo, nil
}

// Close purges the container
func (l *LocalSpiceDbContainer) Close() {
	err := l.pool.Purge(l.container)
	if err != nil {
		log.NewHelper(l.logger).Error("Could not purge SpiceDB Container from test. Please delete manually.")
	}
}
