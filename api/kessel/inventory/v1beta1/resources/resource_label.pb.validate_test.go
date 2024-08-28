package resources

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestResourceLabelValid(t *testing.T) {
	label := ResourceLabel{
		Key:   "key",
		Value: "value",
	}

	err := label.ValidateAll()

	assert.NoError(t, err)
}

func TestResourceLabelEmpty(t *testing.T) {
	label := ResourceLabel{}

	err := label.ValidateAll()

	assert.ErrorContains(t, err, "invalid ResourceLabel.Key")
	assert.ErrorContains(t, err, "invalid ResourceLabel.Value")
}

func TestResourceLabelNoKey(t *testing.T) {
	label := ResourceLabel{
		Value: "value",
	}

	err := label.ValidateAll()

	assert.ErrorContains(t, err, "invalid ResourceLabel.Key")
	assert.NotContains(t, err.Error(), "invalid ResourceLabel.Value")
}

func TestResourceLabelNoValue(t *testing.T) {
	label := ResourceLabel{
		Key: "key",
	}

	err := label.ValidateAll()

	assert.NotContains(t, err.Error(), "invalid ResourceLabel.Key")
	assert.ErrorContains(t, err, "invalid ResourceLabel.Value")
}
