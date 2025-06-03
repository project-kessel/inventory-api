package v1beta2

import (
	"encoding/json"
	"google.golang.org/protobuf/proto"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRequestPagination_GetLimit(t *testing.T) {
	rp := &RequestPagination{Limit: 25}
	assert.Equal(t, uint32(25), rp.GetLimit())

	var rpNil *RequestPagination
	assert.Equal(t, uint32(0), rpNil.GetLimit())
}

func TestRequestPagination_GetContinuationToken(t *testing.T) {
	token := "abc123"
	rp := &RequestPagination{ContinuationToken: &token}
	assert.Equal(t, "abc123", rp.GetContinuationToken())

	rpEmpty := &RequestPagination{}
	assert.Equal(t, "", rpEmpty.GetContinuationToken())

	var rpNil *RequestPagination
	assert.Equal(t, "", rpNil.GetContinuationToken())
}

func TestRequestPagination_Reset(t *testing.T) {
	token := "token"
	rp := &RequestPagination{
		Limit:             42,
		ContinuationToken: &token,
	}
	rp.Reset()
	assert.Equal(t, uint32(0), rp.GetLimit())
	assert.Equal(t, "", rp.GetContinuationToken())
}

func TestRequestPagination_String(t *testing.T) {
	token := "tkn"
	rp := &RequestPagination{
		Limit:             77,
		ContinuationToken: &token,
	}
	s := rp.String()
	assert.NotEmpty(t, s)
	assert.Contains(t, s, "77")
	assert.Contains(t, s, "tkn")
}

func TestRequestPagination_ProtoMessage(t *testing.T) {
	var rp interface{} = &RequestPagination{}
	_, ok := rp.(proto.Message)
	assert.True(t, ok)
}

func TestRequestPagination_ProtoReflect(t *testing.T) {
	token := "t"
	rp := &RequestPagination{Limit: 1, ContinuationToken: &token}
	m := rp.ProtoReflect()
	assert.NotNil(t, m)
	assert.Equal(t, uint32(1), m.Interface().(*RequestPagination).GetLimit())
}

func TestRequestPagination_JSONRoundTrip(t *testing.T) {
	token := "page-3"
	rp := &RequestPagination{
		Limit:             10,
		ContinuationToken: &token,
	}
	b, err := json.Marshal(rp)
	assert.NoError(t, err)

	var out RequestPagination
	assert.NoError(t, json.Unmarshal(b, &out))
	assert.Equal(t, uint32(10), out.GetLimit())
	assert.Equal(t, "page-3", out.GetContinuationToken())
}

func TestRequestPagination_JSONRoundTrip_EmptyToken(t *testing.T) {
	rp := &RequestPagination{
		Limit: 99,
	}
	b, err := json.Marshal(rp)
	assert.NoError(t, err)

	var out RequestPagination
	assert.NoError(t, json.Unmarshal(b, &out))
	assert.Equal(t, uint32(99), out.GetLimit())
	assert.Equal(t, "", out.GetContinuationToken())
}

func TestRequestPagination_AllNil(t *testing.T) {
	var rp *RequestPagination
	assert.Equal(t, uint32(0), rp.GetLimit())
	assert.Equal(t, "", rp.GetContinuationToken())
}
