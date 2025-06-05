package kessel

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestIsValidatedRepresentationType(t *testing.T) {

	assert.True(t, IsValidatedRepresentationType("hbi"))

	assert.False(t, IsValidatedRepresentationType("HBI"))

	// too long
	assert.False(t, IsValidatedRepresentationType("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"))
	// strange characters
	assert.False(t, IsValidatedRepresentationType("h?!!!"))
}

func TestNormalizeRepresentationType(t *testing.T) {

}
