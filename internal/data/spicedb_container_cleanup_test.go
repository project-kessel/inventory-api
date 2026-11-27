package data

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/containerd/errdefs"
	"github.com/moby/moby/client"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

func TestCleanupFailedSpiceDBStartBoundedContext(t *testing.T) {
	requireSpiceDBIntegration(t)
	for _, cleanupFails := range []bool{false, true} {
		name := "success"
		if cleanupFails {
			name = "cleanup-error"
		}
		t.Run(name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
			defer cancel()
			runtime, err := testcontainers.NewDockerClientWithOpts(ctx)
			require.NoError(t, err)
			t.Cleanup(func() { assert.NoError(t, runtime.Close()) })
			cleanupErr := errors.New("intentional termination hook failure")
			var stopBounded, removalBounded bool
			bounded := func(ctx context.Context) bool {
				deadline, ok := ctx.Deadline()
				return ok && ctx.Err() == nil && time.Until(deadline) <= 30*time.Second
			}
			// Real lifecycle hooks observe both phases; a hook error exercises
			// reporting without replacing the runtime or its removal operation.
			partial, startErr := testcontainers.Run(ctx, SpicedbImage+":"+SpicedbVersion,
				testcontainers.WithCmd("serve-testing", "--skip-release-check=true"),
				testcontainers.WithWaitStrategy(wait.ForLog("inventory-test-readiness-marker-never-emitted").WithStartupTimeout(100*time.Millisecond)),
				testcontainers.WithLifecycleHooks(testcontainers.ContainerLifecycleHooks{
					PreStops: []testcontainers.ContainerHook{func(ctx context.Context, _ testcontainers.Container) error {
						stopBounded = bounded(ctx)
						return nil
					}},
					PreTerminates: []testcontainers.ContainerHook{func(ctx context.Context, _ testcontainers.Container) error {
						removalBounded = bounded(ctx)
						if cleanupFails {
							return cleanupErr
						}
						return nil
					}},
				}),
			)
			if partial != nil {
				id := partial.GetContainerID()
				t.Cleanup(func() {
					cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 30*time.Second)
					defer cleanupCancel()
					_, err := runtime.ContainerRemove(cleanupCtx, id, client.ContainerRemoveOptions{Force: true, RemoveVolumes: true})
					if err != nil && !errdefs.IsNotFound(err) {
						t.Errorf("fallback removal of %s: %v", id, err)
					}
				})
			}
			require.Error(t, startErr)
			require.NotNil(t, partial)
			id := partial.GetContainerID()
			_, err = runtime.ContainerInspect(ctx, id, client.ContainerInspectOptions{})
			require.NoError(t, err, "readiness failure must leave a real partial container")
			cancel() // Cleanup must not inherit the failed setup's context.

			err = cleanupFailedSpiceDBStart(partial, startErr)
			assert.ErrorIs(t, err, startErr)
			assert.True(t, stopBounded, "stop must use a fresh bounded context")
			assert.True(t, removalBounded, "removal must use a fresh bounded context")
			if cleanupFails {
				assert.ErrorIs(t, err, cleanupErr)
				assert.ErrorContains(t, err, id)
				assert.ErrorContains(t, err, "remove manually")
			}
			inspectCtx, inspectCancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer inspectCancel()
			_, inspectErr := runtime.ContainerInspect(inspectCtx, id, client.ContainerInspectOptions{})
			assert.True(t, errdefs.IsNotFound(inspectErr), "independent runtime must confirm removal: %v", inspectErr)
		})
	}
}
