package v1beta2

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/proto"
)

func TestCheckForUpdateResponse_GetAllowed_Nil(t *testing.T) {
	var resp *CheckForUpdateResponse
	assert.Equal(t, Allowed_ALLOWED_UNSPECIFIED, resp.GetAllowed())
}

func TestCheckForUpdateResponse_GetAllowed_ZeroStruct(t *testing.T) {
	var resp CheckForUpdateResponse
	assert.Equal(t, Allowed_ALLOWED_UNSPECIFIED, resp.GetAllowed())
}

func TestCheckForUpdateResponse_GetAllowed_True(t *testing.T) {
	resp := &CheckForUpdateResponse{Allowed: Allowed_ALLOWED_TRUE}
	assert.Equal(t, Allowed_ALLOWED_TRUE, resp.GetAllowed())
}

func TestCheckForUpdateResponse_GetAllowed_False(t *testing.T) {
	resp := &CheckForUpdateResponse{Allowed: Allowed_ALLOWED_FALSE}
	assert.Equal(t, Allowed_ALLOWED_FALSE, resp.GetAllowed())
}

func TestCheckForUpdateResponse_Reset(t *testing.T) {
	resp := &CheckForUpdateResponse{Allowed: Allowed_ALLOWED_TRUE}
	resp.Reset()
	assert.Equal(t, Allowed_ALLOWED_UNSPECIFIED, resp.GetAllowed())
}

func TestCheckForUpdateResponse_String(t *testing.T) {
	resp := &CheckForUpdateResponse{Allowed: Allowed_ALLOWED_TRUE}
	s := resp.String()
	assert.NotEmpty(t, s)
	assert.Contains(t, s, "ALLOWED_TRUE")
}

func TestCheckForUpdateResponse_ProtoMessage(t *testing.T) {
	var resp interface{} = &CheckForUpdateResponse{}
	_, ok := resp.(proto.Message)
	assert.True(t, ok)
}

func TestCheckForUpdateResponse_ProtoReflect(t *testing.T) {
	resp := &CheckForUpdateResponse{Allowed: Allowed_ALLOWED_FALSE}
	m := resp.ProtoReflect()
	assert.NotNil(t, m)
	assert.Equal(t, Allowed_ALLOWED_FALSE, m.Interface().(*CheckForUpdateResponse).Allowed)
}

func TestCheckForUpdateResponse_MarshalUnmarshal(t *testing.T) {
	orig := &CheckForUpdateResponse{Allowed: Allowed_ALLOWED_TRUE}
	data, err := proto.Marshal(orig)
	assert.NoError(t, err)

	var decoded CheckForUpdateResponse
	err = proto.Unmarshal(data, &decoded)
	assert.NoError(t, err)
	assert.Equal(t, Allowed_ALLOWED_TRUE, decoded.GetAllowed())
}

func TestCheckForUpdateResponse_AllAllowedValues(t *testing.T) {
	values := []Allowed{
		Allowed_ALLOWED_UNSPECIFIED,
		Allowed_ALLOWED_TRUE,
		Allowed_ALLOWED_FALSE,
	}
	for _, v := range values {
		resp := &CheckForUpdateResponse{Allowed: v}
		assert.Equal(t, v, resp.GetAllowed())
	}
}
