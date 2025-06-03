package v1beta2

import (
	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/proto"
	"testing"
)

func TestCheckResponse_GetAllowed_Nil(t *testing.T) {
	var cr *CheckResponse
	assert.Equal(t, Allowed_ALLOWED_UNSPECIFIED, cr.GetAllowed())
}

func TestCheckResponse_GetAllowed_ZeroStruct(t *testing.T) {
	var cr CheckResponse
	assert.Equal(t, Allowed_ALLOWED_UNSPECIFIED, cr.GetAllowed())
}

func TestCheckResponse_GetAllowed_True(t *testing.T) {
	cr := &CheckResponse{Allowed: Allowed_ALLOWED_TRUE}
	assert.Equal(t, Allowed_ALLOWED_TRUE, cr.GetAllowed())
}

func TestCheckResponse_GetAllowed_False(t *testing.T) {
	cr := &CheckResponse{Allowed: Allowed_ALLOWED_FALSE}
	assert.Equal(t, Allowed_ALLOWED_FALSE, cr.GetAllowed())
}

func TestCheckResponse_Reset(t *testing.T) {
	cr := &CheckResponse{Allowed: Allowed_ALLOWED_TRUE}
	cr.Reset()
	assert.Equal(t, Allowed_ALLOWED_UNSPECIFIED, cr.GetAllowed())
}

func TestCheckResponse_String(t *testing.T) {
	cr := &CheckResponse{Allowed: Allowed_ALLOWED_TRUE}
	s := cr.String()
	assert.NotEmpty(t, s)
	assert.Contains(t, s, "ALLOWED_TRUE")
}

func TestCheckResponse_ProtoReflect(t *testing.T) {
	cr := &CheckResponse{Allowed: Allowed_ALLOWED_FALSE}
	m := cr.ProtoReflect()
	assert.NotNil(t, m)
	assert.Equal(t, Allowed_ALLOWED_FALSE, m.Interface().(*CheckResponse).Allowed)
}

func TestCheckResponse_ProtoMessage(t *testing.T) {
	var cr interface{} = &CheckResponse{}
	_, ok := cr.(proto.Message)
	assert.True(t, ok)
}

func TestCheckResponse_MarshalUnmarshal(t *testing.T) {
	orig := &CheckResponse{Allowed: Allowed_ALLOWED_TRUE}
	data, err := proto.Marshal(orig)
	assert.NoError(t, err)

	var decoded CheckResponse
	err = proto.Unmarshal(data, &decoded)
	assert.NoError(t, err)
	assert.Equal(t, Allowed_ALLOWED_TRUE, decoded.GetAllowed())
}

// Test all possible Allowed values
func TestCheckResponse_AllAllowedValues(t *testing.T) {
	values := []Allowed{
		Allowed_ALLOWED_UNSPECIFIED,
		Allowed_ALLOWED_TRUE,
		Allowed_ALLOWED_FALSE,
	}
	for _, v := range values {
		cr := &CheckResponse{Allowed: v}
		assert.Equal(t, v, cr.GetAllowed())
	}
}
