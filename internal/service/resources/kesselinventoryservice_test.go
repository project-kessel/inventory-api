package resources

import (
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsValidatedRepresentationType(t *testing.T) {

	assert.True(t, IsValidType("hbi"))

	// capatalised type not normalized
	assert.False(t, IsValidType("HBI"))

	// normalize then validate
	normalized := normalizeType("HBI")
	assert.True(t, IsValidType(normalized))
	// too long
	assert.False(t, IsValidType("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"))
	// strange characters
	assert.False(t, IsValidType("h?!!!"))
}

func TestNormalizeRepresentationType(t *testing.T) {
	// normalize then validate
	normalized := normalizeType("HBI")
	assert.True(t, IsValidType(normalized))

	assert.Equal(t, "hbi", normalized)
}

var typePattern = regexp.MustCompile(`^([a-z][a-z0-9_]{1,61}[a-z0-9]/)*[a-z][a-z0-9_]{1,62}[a-z0-9]$`)

func IsValidType(val string) bool {
	return typePattern.MatchString(val)
}
