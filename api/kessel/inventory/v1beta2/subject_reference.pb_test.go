package v1beta2

import (
	"encoding/json"
	"google.golang.org/protobuf/proto"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSubjectReference_GetRelation_WithRelation(t *testing.T) {
	val := "view"
	sr := &SubjectReference{Relation: &val}
	assert.Equal(t, "view", sr.GetRelation())
}

func TestSubjectReference_GetRelation_NilRelation(t *testing.T) {
	sr := &SubjectReference{}
	assert.Equal(t, "", sr.GetRelation())
}

func TestSubjectReference_GetResource_NonNil(t *testing.T) {
	sr := &SubjectReference{
		Resource: &ResourceReference{ResourceType: "principal", ResourceId: "sarah"},
	}
	assert.NotNil(t, sr.GetResource())
	assert.Equal(t, "principal", sr.GetResource().ResourceType)
}

func TestSubjectReference_GetResource_Nil(t *testing.T) {
	sr := &SubjectReference{}
	assert.Nil(t, sr.GetResource())
}

func TestSubjectReference_Reset(t *testing.T) {
	val := "view"
	sr := &SubjectReference{
		Relation: &val,
		Resource: &ResourceReference{ResourceType: "host"},
	}
	sr.Reset()
	assert.Nil(t, sr.Relation)
	assert.Nil(t, sr.Resource)
}

func TestSubjectReference_String(t *testing.T) {
	val := "view"
	sr := &SubjectReference{
		Relation: &val,
		Resource: &ResourceReference{ResourceType: "host", ResourceId: "789"},
	}
	s := sr.String()
	assert.NotEmpty(t, s)
	assert.Contains(t, s, "view")
	assert.Contains(t, s, "host")
}

func TestSubjectReference_ProtoMessage(t *testing.T) {
	var sr interface{} = &SubjectReference{}
	_, ok := sr.(proto.Message)
	assert.True(t, ok)
}

func TestSubjectReference_ProtoReflect(t *testing.T) {
	val := "view"
	sr := &SubjectReference{Relation: &val}
	m := sr.ProtoReflect()
	assert.NotNil(t, m)
	assert.Equal(t, "view", m.Interface().(*SubjectReference).GetRelation())
}

func TestSubjectReference_JSONRoundTrip_WithRelation(t *testing.T) {
	val := "view"
	sr := &SubjectReference{
		Relation: &val,
		Resource: &ResourceReference{ResourceType: "principal", ResourceId: "sarah"},
	}
	b, err := json.Marshal(sr)
	assert.NoError(t, err)

	var out SubjectReference
	assert.NoError(t, json.Unmarshal(b, &out))
	assert.Equal(t, "view", out.GetRelation())
	assert.Equal(t, "principal", out.GetResource().ResourceType)
	assert.Equal(t, "sarah", out.GetResource().ResourceId)
}

func TestSubjectReference_JSONRoundTrip_WithoutRelation(t *testing.T) {
	sr := &SubjectReference{
		Resource: &ResourceReference{ResourceType: "principal", ResourceId: "xyz"},
	}
	b, err := json.Marshal(sr)
	assert.NoError(t, err)

	var out SubjectReference
	assert.NoError(t, json.Unmarshal(b, &out))
	assert.Equal(t, "", out.GetRelation())
	assert.Equal(t, "principal", out.GetResource().ResourceType)
	assert.Equal(t, "xyz", out.GetResource().ResourceId)
}

func TestSubjectReference_AllNil(t *testing.T) {
	var sr *SubjectReference
	assert.Equal(t, "", sr.GetRelation())
	assert.Nil(t, sr.GetResource())
}
