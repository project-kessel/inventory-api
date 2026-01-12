package v1beta2

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/proto"
)

func TestCheckSelfRequest_Nil(t *testing.T) {
	var cr *CheckSelfRequest
	assert.Nil(t, cr.GetObject())
	assert.Equal(t, "", cr.GetRelation())
	assert.Nil(t, cr.GetConsistencyToken())
}

func TestCheckSelfRequest_ZeroStruct(t *testing.T) {
	var cr CheckSelfRequest
	assert.Nil(t, cr.GetObject())
	assert.Equal(t, "", cr.GetRelation())
	assert.Nil(t, cr.GetConsistencyToken())
}

func TestCheckSelfRequest_Reset(t *testing.T) {
	cr := &CheckSelfRequest{
		Object:   &ResourceReference{ResourceType: "host", ResourceId: "123"},
		Relation: "view",
	}
	cr.Reset()
	assert.Nil(t, cr.Object)
	assert.Equal(t, "", cr.Relation)
}

func TestCheckSelfRequest_String(t *testing.T) {
	cr := &CheckSelfRequest{
		Object:   &ResourceReference{ResourceType: "host"},
		Relation: "view",
	}
	s := cr.String()
	assert.NotEmpty(t, s)
	assert.Contains(t, s, "host")
	assert.Contains(t, s, "view")
}

func TestCheckSelfRequest_ProtoMessage(t *testing.T) {
	var cr interface{} = &CheckSelfRequest{}
	_, ok := cr.(proto.Message)
	assert.True(t, ok)
}

func TestCheckSelfRequest_ProtoReflect(t *testing.T) {
	cr := &CheckSelfRequest{
		Object:   &ResourceReference{ResourceType: "host"},
		Relation: "view",
	}
	m := cr.ProtoReflect()
	assert.NotNil(t, m)
	assert.Equal(t, "view", m.Interface().(*CheckSelfRequest).Relation)
}
