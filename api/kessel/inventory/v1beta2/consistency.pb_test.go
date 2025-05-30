package v1beta2

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/proto"
)

func TestConsistency_MinimizeLatency(t *testing.T) {
	cons := &Consistency{
		Requirement: &Consistency_MinimizeLatency{MinimizeLatency: true},
	}
	assert.True(t, cons.GetMinimizeLatency())
	assert.Nil(t, cons.GetAtLeastAsFresh())
}

func TestConsistency_AtLeastAsFresh(t *testing.T) {
	ct := &ConsistencyToken{Token: "snap"}
	cons := &Consistency{
		Requirement: &Consistency_AtLeastAsFresh{AtLeastAsFresh: ct},
	}
	assert.False(t, cons.GetMinimizeLatency())
	assert.NotNil(t, cons.GetAtLeastAsFresh())
	assert.Equal(t, "snap", cons.GetAtLeastAsFresh().GetToken())
}

func TestConsistency_GetMinimizeLatency_FalseDefault(t *testing.T) {
	cons := &Consistency{}
	assert.False(t, cons.GetMinimizeLatency())
}

func TestConsistency_GetAtLeastAsFresh_NilDefault(t *testing.T) {
	cons := &Consistency{}
	assert.Nil(t, cons.GetAtLeastAsFresh())
}

func TestConsistency_GetRequirement_NilStruct(t *testing.T) {
	var cons *Consistency
	assert.Nil(t, cons.GetRequirement())
	assert.False(t, cons.GetMinimizeLatency())
	assert.Nil(t, cons.GetAtLeastAsFresh())
}

func TestConsistency_Reset(t *testing.T) {
	cons := &Consistency{
		Requirement: &Consistency_MinimizeLatency{MinimizeLatency: true},
	}
	cons.Reset()
	assert.Nil(t, cons.GetRequirement())
	assert.False(t, cons.GetMinimizeLatency())
}

func TestConsistency_ProtoMessage(t *testing.T) {
	var cons interface{} = &Consistency{}
	_, ok := cons.(proto.Message)
	assert.True(t, ok)
}

func TestConsistency_ProtoReflect(t *testing.T) {
	cons := &Consistency{
		Requirement: &Consistency_MinimizeLatency{MinimizeLatency: true},
	}
	m := cons.ProtoReflect()
	assert.NotNil(t, m)
	ci, ok := m.Interface().(*Consistency)
	assert.True(t, ok)
	assert.True(t, ci.GetMinimizeLatency())
}

func TestConsistency_JSONRoundTrip_Empty(t *testing.T) {
	cons := &Consistency{}
	data, err := json.Marshal(cons)
	assert.NoError(t, err)

	var out Consistency
	assert.NoError(t, json.Unmarshal(data, &out))
	assert.Nil(t, out.GetRequirement())
}
