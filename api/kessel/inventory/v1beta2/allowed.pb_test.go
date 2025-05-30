package v1beta2

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAllowedEnumValues(t *testing.T) {
	assert.Equal(t, int32(0), int32(Allowed_ALLOWED_UNSPECIFIED))
	assert.Equal(t, int32(1), int32(Allowed_ALLOWED_TRUE))
	assert.Equal(t, int32(2), int32(Allowed_ALLOWED_FALSE))
}

func TestAllowedStringer(t *testing.T) {
	assert.Equal(t, "ALLOWED_UNSPECIFIED", Allowed_ALLOWED_UNSPECIFIED.String())
	assert.Equal(t, "ALLOWED_TRUE", Allowed_ALLOWED_TRUE.String())
	assert.Equal(t, "ALLOWED_FALSE", Allowed_ALLOWED_FALSE.String())
}

func TestAllowedEnumMaps(t *testing.T) {
	assert.Equal(t, "ALLOWED_UNSPECIFIED", Allowed_name[int32(Allowed_ALLOWED_UNSPECIFIED)])
	assert.Equal(t, "ALLOWED_TRUE", Allowed_name[int32(Allowed_ALLOWED_TRUE)])
	assert.Equal(t, "ALLOWED_FALSE", Allowed_name[int32(Allowed_ALLOWED_FALSE)])

	assert.Equal(t, int32(0), Allowed_value["ALLOWED_UNSPECIFIED"])
	assert.Equal(t, int32(1), Allowed_value["ALLOWED_TRUE"])
	assert.Equal(t, int32(2), Allowed_value["ALLOWED_FALSE"])
}

func TestAllowedEnumMethod(t *testing.T) {
	enumPtr := Allowed_ALLOWED_TRUE.Enum()
	assert.NotNil(t, enumPtr)
	assert.Equal(t, Allowed_ALLOWED_TRUE, *enumPtr)
}
