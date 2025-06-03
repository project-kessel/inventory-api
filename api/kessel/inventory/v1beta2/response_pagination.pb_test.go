package v1beta2

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/proto"
)

func TestResponsePagination_GetContinuationToken_NonEmpty(t *testing.T) {
	rp := &ResponsePagination{ContinuationToken: "foobar"}
	assert.Equal(t, "foobar", rp.GetContinuationToken())
}

func TestResponsePagination_GetContinuationToken_Empty(t *testing.T) {
	rp := &ResponsePagination{}
	assert.Equal(t, "", rp.GetContinuationToken())
}

func TestResponsePagination_Reset(t *testing.T) {
	rp := &ResponsePagination{ContinuationToken: "abc"}
	rp.Reset()
	assert.Equal(t, "", rp.ContinuationToken)
}

func TestResponsePagination_String(t *testing.T) {
	rp := &ResponsePagination{ContinuationToken: "token"}
	s := rp.String()
	assert.NotEmpty(t, s)
	assert.Contains(t, s, "token")
}

func TestResponsePagination_ProtoMessage(t *testing.T) {
	var msg interface{} = &ResponsePagination{}
	_, ok := msg.(proto.Message)
	assert.True(t, ok)
}

func TestResponsePagination_ProtoReflect(t *testing.T) {
	rp := &ResponsePagination{ContinuationToken: "foo"}
	m := rp.ProtoReflect()
	assert.NotNil(t, m)
	assert.Equal(t, "foo", m.Interface().(*ResponsePagination).GetContinuationToken())
}

func TestResponsePagination_JSONRoundTrip(t *testing.T) {
	rp := &ResponsePagination{ContinuationToken: "json-token"}
	b, err := json.Marshal(rp)
	assert.NoError(t, err)

	var out ResponsePagination
	assert.NoError(t, json.Unmarshal(b, &out))
	assert.Equal(t, "json-token", out.GetContinuationToken())
}

func TestResponsePagination_AllNil(t *testing.T) {
	var rp *ResponsePagination
	assert.Equal(t, "", rp.GetContinuationToken())
}
