package v1beta2

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/proto"
)

// delete resource has an empty response anyway
func TestDeleteResourceResponse_Reset(t *testing.T) {
	resp := &DeleteResourceResponse{}
	resp.Reset()
	assert.True(t, proto.Equal(&DeleteResourceResponse{}, resp))
}

func TestDeleteResourceResponse_ProtoMessage(t *testing.T) {
	var msg interface{} = &DeleteResourceResponse{}
	_, ok := msg.(proto.Message)
	assert.True(t, ok)
}

func TestDeleteResourceResponse_ProtoReflect(t *testing.T) {
	resp := &DeleteResourceResponse{}
	m := resp.ProtoReflect()
	assert.NotNil(t, m)
	assert.Equal(t, resp, m.Interface().(*DeleteResourceResponse))
}
