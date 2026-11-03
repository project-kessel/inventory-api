package data

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	v1 "github.com/authzed/authzed-go/proto/authzed/api/v1"
	"github.com/authzed/authzed-go/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
)

type transientFailureSchemaClient struct {
	v1.SchemaServiceClient

	mu       sync.Mutex
	calls    int
	firstErr error
}

type blockingSchemaClient struct {
	v1.SchemaServiceClient

	mu           sync.Mutex
	calls        int
	writeStarted chan struct{}
	releaseWrite chan struct{}
	startedOnce  sync.Once
}

func (c *blockingSchemaClient) WriteSchema(
	_ context.Context,
	_ *v1.WriteSchemaRequest,
	_ ...grpc.CallOption,
) (*v1.WriteSchemaResponse, error) {
	c.mu.Lock()
	c.calls++
	c.mu.Unlock()

	c.startedOnce.Do(func() { close(c.writeStarted) })
	<-c.releaseWrite
	return &v1.WriteSchemaResponse{}, nil
}

func (c *blockingSchemaClient) callCount() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.calls
}

func (c *transientFailureSchemaClient) WriteSchema(
	_ context.Context,
	_ *v1.WriteSchemaRequest,
	_ ...grpc.CallOption,
) (*v1.WriteSchemaResponse, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.calls++
	if c.calls == 1 {
		return nil, c.firstErr
	}
	return &v1.WriteSchemaResponse{}, nil
}

func (c *transientFailureSchemaClient) callCount() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.calls
}

func TestSpiceDBRelationsRepositoryInitializeRetriesAfterTransientFailure(t *testing.T) {
	serializationFailure := errors.New("unable to write namespace config: SQLSTATE 40001")
	schemaClient := &transientFailureSchemaClient{firstErr: serializationFailure}

	schemaPath := filepath.Join(t.TempDir(), "schema.zed")
	require.NoError(t, os.WriteFile(schemaPath, []byte("definition user {}"), 0o600))

	repository := &SpiceDBRelationsRepository{
		client: &authzed.Client{
			SchemaServiceClient: schemaClient,
		},
		schemaFilePath: schemaPath,
		initGate:       make(chan struct{}, 1),
	}

	require.ErrorIs(t, repository.initialize(context.Background()), serializationFailure)
	assert.NoError(t, repository.initialize(context.Background()))
	assert.Equal(t, 2, schemaClient.callCount())
}

func TestSpiceDBRelationsRepositoryInitializeSerializesConcurrentCalls(t *testing.T) {
	schemaClient := &blockingSchemaClient{
		writeStarted: make(chan struct{}),
		releaseWrite: make(chan struct{}),
	}

	schemaPath := filepath.Join(t.TempDir(), "schema.zed")
	require.NoError(t, os.WriteFile(schemaPath, []byte("definition user {}"), 0o600))

	repository := &SpiceDBRelationsRepository{
		client: &authzed.Client{
			SchemaServiceClient: schemaClient,
		},
		schemaFilePath: schemaPath,
		initGate:       make(chan struct{}, 1),
	}

	const callers = 20
	start := make(chan struct{})
	results := make(chan error, callers)
	for range callers {
		go func() {
			<-start
			results <- repository.initialize(context.Background())
		}()
	}

	close(start)
	select {
	case <-schemaClient.writeStarted:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for schema write")
	}
	close(schemaClient.releaseWrite)

	for range callers {
		assert.NoError(t, <-results)
	}
	assert.Equal(t, 1, schemaClient.callCount())
}

func TestSpiceDBRelationsRepositoryInitializeWaitingRespectsContext(t *testing.T) {
	schemaClient := &blockingSchemaClient{
		writeStarted: make(chan struct{}),
		releaseWrite: make(chan struct{}),
	}

	schemaPath := filepath.Join(t.TempDir(), "schema.zed")
	require.NoError(t, os.WriteFile(schemaPath, []byte("definition user {}"), 0o600))

	repository := &SpiceDBRelationsRepository{
		client: &authzed.Client{
			SchemaServiceClient: schemaClient,
		},
		schemaFilePath: schemaPath,
		initGate:       make(chan struct{}, 1),
	}

	var releaseOnce sync.Once
	releaseWrite := func() {
		releaseOnce.Do(func() { close(schemaClient.releaseWrite) })
	}
	defer releaseWrite()

	firstResult := make(chan error, 1)
	go func() {
		firstResult <- repository.initialize(context.Background())
	}()

	select {
	case <-schemaClient.writeStarted:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for schema write")
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	secondResult := make(chan error, 1)
	go func() {
		secondResult <- repository.initialize(ctx)
	}()

	select {
	case err := <-secondResult:
		require.ErrorIs(t, err, context.Canceled)
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for canceled initialization")
	}
	assert.Equal(t, 1, schemaClient.callCount())

	releaseWrite()
	assert.NoError(t, <-firstResult)
}
