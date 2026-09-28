package inmemory

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestOptionsValidateUnifiedYAMLRepository(t *testing.T) {
	options := NewOptions()
	options.Type = UnifiedYAMLRepository
	options.Path = "/tmp/unified-schemas"

	assert.Empty(t, options.Validate())
}

func TestOptionsValidateUnifiedYAMLRepositoryRequiresPath(t *testing.T) {
	options := NewOptions()
	options.Type = UnifiedYAMLRepository

	errors := options.Validate()

	if assert.Len(t, errors, 1) {
		assert.Contains(t, errors[0].Error(), "path is required")
	}
}
