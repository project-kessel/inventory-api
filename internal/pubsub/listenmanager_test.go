package pubsub

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/stretchr/testify/assert"

	"github.com/project-kessel/inventory-api/cmd/common"
)

func TestSubscriptions(t *testing.T) {
	ctx := context.Background()
	_, logger := common.InitLogger("info", common.LoggerOptions{})
	driverMock := &DriverMock{}
	listenManager := NewListenManager(log.NewHelper(log.With(logger, "subsystem", "listenmanager_test")), driverMock)
	subscription := listenManager.Subscribe(mockPayload)
	assert.NotNil(t, subscription)
	assert.Equal(t, 1, len(listenManager.subscriptions))

	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		defer wg.Done()
		defer subscription.Unsubscribe()

		err := subscription.BlockForNotification(ctx)
		assert.NoError(t, err)
	}()

	err := listenManager.WaitAndDistribute(ctx)
	assert.NoError(t, err)

	wg.Wait()
	assert.Equal(t, 0, len(listenManager.subscriptions))
}

func TestWaitTimeout(t *testing.T) {
	_, logger := common.InitLogger("info", common.LoggerOptions{})
	driverMock := &DriverMock{}
	listenManager := NewListenManager(log.NewHelper(log.With(logger, "subsystem", "listenmanager_test")), driverMock)
	subscription := listenManager.Subscribe(mockPayload)
	assert.NotNil(t, subscription)
	assert.Equal(t, 1, len(listenManager.subscriptions))

	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		defer wg.Done()
		defer subscription.Unsubscribe()
		// Simulate a timeout
		timeoutCtx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		err := subscription.BlockForNotification(timeoutCtx)
		assert.Error(t, err)
		assert.Equal(t, "context cancelled while waiting for notification context deadline exceeded", err.Error())
	}()

	wg.Wait()
	assert.Equal(t, 0, len(listenManager.subscriptions))
}
