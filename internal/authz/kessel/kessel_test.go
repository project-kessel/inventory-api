package kessel

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestIsValidatedRepresentationType(t *testing.T) {

	assert.True(t, IsValidatedRepresentationType("hbi"))

	// capatalised type not normalized
	assert.False(t, IsValidatedRepresentationType("HBI"))

	// normalize then validate
	normalized := NormalizeRepresentationType("HBI")
	assert.True(t, IsValidatedRepresentationType(normalized))
	// too long
	assert.False(t, IsValidatedRepresentationType("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"))
	// strange characters
	assert.False(t, IsValidatedRepresentationType("h?!!!"))
}

func TestNormalizeRepresentationType(t *testing.T) {
	// normalize then validate
	normalized := NormalizeRepresentationType("HBI")
	assert.True(t, IsValidatedRepresentationType(normalized))

	assert.Equal(t, "hbi", normalized)
}
