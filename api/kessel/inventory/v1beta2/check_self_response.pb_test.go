package v1beta2

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/proto"
)

func TestCheckSelfResponse_Nil(t *testing.T) {
	var cr *CheckSelfResponse
	assert.Equal(t, Allowed_ALLOWED_UNSPECIFIED, cr.GetAllowed())
	assert.Nil(t, cr.GetConsistencyToken())
}

func TestCheckSelfResponse_ZeroStruct(t *testing.T) {
	var cr CheckSelfResponse
	assert.Equal(t, Allowed_ALLOWED_UNSPECIFIED, cr.GetAllowed())
	assert.Nil(t, cr.GetConsistencyToken())
}

func TestCheckSelfResponse_GetAllowed(t *testing.T) {
	tests := []struct {
		name    string
		allowed Allowed
	}{
		{"unspecified", Allowed_ALLOWED_UNSPECIFIED},
		{"true", Allowed_ALLOWED_TRUE},
		{"false", Allowed_ALLOWED_FALSE},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cr := &CheckSelfResponse{Allowed: tt.allowed}
			assert.Equal(t, tt.allowed, cr.GetAllowed())
		})
	}
}

func TestCheckSelfResponse_Reset(t *testing.T) {
	cr := &CheckSelfResponse{Allowed: Allowed_ALLOWED_TRUE}
	cr.Reset()
	assert.Equal(t, Allowed_ALLOWED_UNSPECIFIED, cr.GetAllowed())
}

func TestCheckSelfResponse_String(t *testing.T) {
	cr := &CheckSelfResponse{Allowed: Allowed_ALLOWED_TRUE}
	s := cr.String()
	assert.NotEmpty(t, s)
	assert.Contains(t, s, "ALLOWED_TRUE")
}

func TestCheckSelfResponse_ProtoMessage(t *testing.T) {
	var cr interface{} = &CheckSelfResponse{}
	_, ok := cr.(proto.Message)
	assert.True(t, ok)
}

func TestCheckSelfResponse_ProtoReflect(t *testing.T) {
	cr := &CheckSelfResponse{Allowed: Allowed_ALLOWED_FALSE}
	m := cr.ProtoReflect()
	assert.NotNil(t, m)
	assert.Equal(t, Allowed_ALLOWED_FALSE, m.Interface().(*CheckSelfResponse).Allowed)
}
