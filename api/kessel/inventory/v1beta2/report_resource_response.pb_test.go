package v1beta2

import (
	"testing"

	"google.golang.org/protobuf/proto"

	"github.com/stretchr/testify/assert"
)

func TestReportResourceResponse_Reset(t *testing.T) {
	resp := &ReportResourceResponse{}
	resp.Reset()
	assert.NotNil(t, resp)
}

func TestReportResourceResponse_ProtoMessage(t *testing.T) {
	var resp interface{} = &ReportResourceResponse{}
	_, ok := resp.(proto.Message)
	assert.True(t, ok)
}

func TestReportResourceResponse_ProtoReflect(t *testing.T) {
	resp := &ReportResourceResponse{}
	m := resp.ProtoReflect()
	assert.NotNil(t, m)
	assert.Contains(t, "ReportResourceResponse", m.Descriptor().Name())
}

func TestReportResourceResponse_AllNil(t *testing.T) {
	var resp *ReportResourceResponse
	// resp is nil
	assert.Nil(t, resp)
}
