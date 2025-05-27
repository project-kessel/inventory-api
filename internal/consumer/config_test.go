package consumer

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewConfig(t *testing.T) {
	o := NewOptions()
	config := NewConfig(o)
	assert.NotNil(t, config.Options)
	assert.NotNil(t, config.AuthConfig)
	assert.NotNil(t, config.AuthConfig.Options)
	assert.NotNil(t, config.RetryConfig)
	assert.NotNil(t, config.RetryConfig.Options)
}

func TestConfig_Complete(t *testing.T) {
	o := NewOptions()
	config := NewConfig(o)
	completed, errs := config.Complete()
	assert.Nil(t, errs)
	assert.NotNil(t, completed.Options)
	assert.NotNil(t, completed.Topic)
	assert.NotNil(t, completed.KafkaConfig)
	assert.NotNil(t, completed.RetryConfig)
	assert.NotNil(t, completed.AuthConfig)
	assert.NotNil(t, completed.ReadAfterWriteEnabled)
	assert.NotNil(t, completed.ReadAfterWriteAllowlist)
}
