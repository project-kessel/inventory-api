package data

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/project-kessel/inventory-api/internal/biz/model"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/network"
	"github.com/testcontainers/testcontainers-go/wait"
)

const (
	// SpicedbImage is the image used for containerized spiceDB in tests
	SpicedbImage = "authzed/spicedb"
	// SpicedbVersion is the image version used for containerized spiceDB in tests
	SpicedbVersion = "v1.47.1"
	// SpicedbSchemaBootstrapFile specifies an optional bootstrap schema file to be used for testing
	SpicedbSchemaBootstrapFile = "test/testdata/spicedb/basic_schema.zed"
	// FullyConsistent specifices the consistency mode used for our read API calls
	FullyConsistent = false
)

const spicedbNetworkAliasPrefix = "spicedb"

// uniqueNetworkAlias returns a network alias unique per container to avoid collisions when multiple SpiceDB containers run in parallel (e.g. parallel tests).
func uniqueNetworkAlias() string {
	b := make([]byte, 4)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("%s-%d", spicedbNetworkAliasPrefix, time.Now().UnixNano())
	}
	return fmt.Sprintf("%s-%x", spicedbNetworkAliasPrefix, b)
}

// LocalSpiceDbContainer struct that holds the testcontainers container and exposes the port
type LocalSpiceDbContainer struct {
	logger         log.Logger
	port           string
	container      *testcontainers.DockerContainer
	name           string
	networkAlias   string
	schemaLocation string
}

// ContainerOptions configures SpiceDB container creation
type ContainerOptions struct {
	Logger  log.Logger
	Network *testcontainers.DockerNetwork
}

// CreateContainer creates a new SpiceDbContainer using testcontainers-go
func CreateContainer(ctx context.Context, opts *ContainerOptions) (*LocalSpiceDbContainer, error) {
	var (
		_, b, _, _ = runtime.Caller(0)
		basepath   = filepath.Dir(b)
	)

	// Navigate up to project root to find test data
	projectRoot := filepath.Join(basepath, "../..")
	schemaPath := filepath.Join(projectRoot, SpicedbSchemaBootstrapFile)

	image := SpicedbImage + ":" + SpicedbVersion
	runOpts := []testcontainers.ContainerCustomizer{
		testcontainers.WithCmd("serve-testing", "--skip-release-check=true"),
		testcontainers.WithExposedPorts("50051/tcp", "50052/tcp"),
		testcontainers.WithWaitStrategy(
			wait.ForListeningPort("50051/tcp").WithStartupTimeout(3 * time.Minute),
		),
	}

	var networkAlias string
	if opts.Network != nil {
		networkAlias = uniqueNetworkAlias()
		runOpts = append(runOpts,
			network.WithNetwork([]string{networkAlias}, opts.Network),
		)
	}

	ctr, err := testcontainers.Run(ctx, image, runOpts...)
	if err != nil {
		return nil, fmt.Errorf("could not start spicedb resource: %w", err)
	}

	port, err := ctr.MappedPort(ctx, "50051")
	if err != nil {
		_ = testcontainers.TerminateContainer(ctr)
		return nil, fmt.Errorf("could not get spicedb port: %w", err)
	}

	inspect, err := ctr.Inspect(ctx)
	if err != nil {
		_ = testcontainers.TerminateContainer(ctr)
		return nil, fmt.Errorf("could not inspect spicedb container: %w", err)
	}
	name := inspect.Name

	alias := ""
	if opts.Network != nil {
		alias = networkAlias
	}

	return &LocalSpiceDbContainer{
		name:           name,
		networkAlias:   alias,
		logger:         opts.Logger,
		port:           port.Port(),
		container:      ctr,
		schemaLocation: schemaPath,
	}, nil
}

// Port returns the port the container is listening on (host-mapped)
func (l *LocalSpiceDbContainer) Port() string {
	return l.port
}

// Name returns the actual container name assigned by Docker
func (l *LocalSpiceDbContainer) Name() string {
	return l.name
}

// NetworkAlias returns the network alias used for inter-container communication,
// or empty if no network is configured
func (l *LocalSpiceDbContainer) NetworkAlias() string {
	return l.networkAlias
}

// NewToken returns a new token used for the container so a new store is created in serve-testing
func (l *LocalSpiceDbContainer) NewToken() (string, error) {
	buf := make([]byte, 20)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}

	return base64.StdEncoding.EncodeToString(buf), nil
}

// WaitForQuantizationInterval needed to avoid read-before-write when loading the schema
func (l *LocalSpiceDbContainer) WaitForQuantizationInterval() {
	if !FullyConsistent {
		time.Sleep(10 * time.Millisecond)
	}
}

// SpiceDBConfig holds configuration for creating a SpiceDB repository
type SpiceDBConfig struct {
	Endpoint        string
	Token           string
	TokenFile       string
	SchemaFile      string
	UseTLS          bool
	FullyConsistent bool
}

// CreateSpiceDbRepository creates a repository that connects to the containerized SpiceDB instance
func (l *LocalSpiceDbContainer) CreateSpiceDbRepository() (*SpiceDBRelationsRepository, error) {
	randomKey, err := l.NewToken()
	if err != nil {
		return nil, err
	}

	tmpDir, err := os.MkdirTemp("", "inventory-api")
	if err != nil {
		return nil, err
	}
	tmpFile, err := os.CreateTemp(tmpDir, "spicedbpreshared")
	if err != nil {
		return nil, err
	}

	// Write token directly to the open file
	if _, err := tmpFile.Write([]byte(randomKey)); err != nil {
		tmpFile.Close()
		return nil, err
	}
	tmpFileName := tmpFile.Name()
	if err := tmpFile.Close(); err != nil {
		return nil, err
	}

	defer func() {
		if err := os.RemoveAll(tmpDir); err != nil {
			log.NewHelper(l.logger).Errorf("error removing temporary directory: %v", err)
		}
	}()

	config := &SpiceDBConfig{
		UseTLS:          false,
		Endpoint:        "localhost:" + l.port,
		TokenFile:       tmpFileName,
		SchemaFile:      l.schemaLocation,
		FullyConsistent: FullyConsistent,
	}

	repo, _, err := NewSpiceDBRelationsRepository(config, l.logger)
	if err != nil {
		return nil, err
	}

	return repo, nil
}

// Close terminates the container
func (l *LocalSpiceDbContainer) Close() {
	if err := testcontainers.TerminateContainer(l.container); err != nil {
		log.NewHelper(l.logger).Errorf("Could not terminate SpiceDB container, please remove manually: %v", err)
	}
}

// CheckForRelationship returns true if the given subject has the given relationship to the given resource, otherwise false
func CheckForRelationship(
	client model.RelationsRepository,
	subjectID string,
	subjectNamespace string,
	subjectType string,
	subjectRelationship string,
	relationship string,
	resourceNamespace string,
	resourceType string,
	resourceID string,
	consistency model.Consistency,
) bool {
	ctx := context.TODO()

	// Build filter using builder pattern
	filter := model.NewTupleFilter().
		WithObjectType(model.DeserializeResourceType(resourceType)).
		WithObjectId(model.DeserializeLocalResourceId(resourceID)).
		WithRelation(model.DeserializeRelation(relationship))

	// Add reporter if provided
	if resourceNamespace != "" {
		filter = filter.WithReporterType(model.DeserializeReporterType(resourceNamespace))
	}

	// Build subject filter
	subjectFilter := model.NewTupleSubjectFilter().
		WithSubjectType(model.DeserializeResourceType(subjectType)).
		WithSubjectId(model.DeserializeLocalResourceId(subjectID))

	if subjectRelationship != "" {
		subjectFilter = subjectFilter.WithRelation(model.DeserializeRelation(subjectRelationship))
	}

	if subjectNamespace != "" {
		subjectFilter = subjectFilter.WithReporterType(model.DeserializeReporterType(subjectNamespace))
	}

	filter = filter.WithSubject(subjectFilter)

	stream, err := client.ReadTuples(ctx, filter, model.NewPagination(1, nil), consistency)
	if err != nil {
		panic(err)
	}

	// Read first item from stream
	_, err = stream.Recv()
	if err != nil {
		if errors.Is(err, io.EOF) {
			return false
		}
		panic(err)
	}

	return true
}
