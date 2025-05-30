package v1beta2

import (
	"encoding/json"
	"google.golang.org/protobuf/proto"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestConsistencyToken_GetToken(t *testing.T) {
	ct := &ConsistencyToken{Token: "abc123"}
	assert.Equal(t, "abc123", ct.GetToken())
}

func TestConsistencyToken_GetToken_Empty(t *testing.T) {
	ct := &ConsistencyToken{}
	assert.Equal(t, "", ct.GetToken())
}

func TestConsistencyToken_GetToken_NilStruct(t *testing.T) {
	var ct *ConsistencyToken
	assert.Equal(t, "", ct.GetToken())
}

func TestConsistencyToken_Reset(t *testing.T) {
	ct := &ConsistencyToken{Token: "something"}
	ct.Reset()
	assert.Equal(t, "", ct.Token)
}

func TestConsistencyToken_String(t *testing.T) {
	ct := &ConsistencyToken{Token: "foo"}
	s := ct.String()
	assert.NotEmpty(t, s)
	assert.Contains(t, s, "foo")
}

func TestConsistencyToken_ProtoMessage(t *testing.T) {
	var ct interface{} = &ConsistencyToken{}
	_, ok := ct.(proto.Message)
	assert.True(t, ok)
}

func TestConsistencyToken_ProtoReflect(t *testing.T) {
	ct := &ConsistencyToken{Token: "foo"}
	m := ct.ProtoReflect()
	assert.NotNil(t, m)
	assert.Equal(t, "foo", m.Interface().(*ConsistencyToken).GetToken())
}

func TestConsistencyToken_JSONRoundTrip(t *testing.T) {
	ct := &ConsistencyToken{Token: "bar"}
	b, err := json.Marshal(ct)
	assert.NoError(t, err)

	var out ConsistencyToken
	assert.NoError(t, json.Unmarshal(b, &out))
	assert.Equal(t, "bar", out.GetToken())
}

func TestConsistencyToken_JSONRoundTrip_Empty(t *testing.T) {
	ct := &ConsistencyToken{}
	b, err := json.Marshal(ct)
	assert.NoError(t, err)

	var out ConsistencyToken
	assert.NoError(t, json.Unmarshal(b, &out))
	assert.Equal(t, "", out.GetToken())
}
