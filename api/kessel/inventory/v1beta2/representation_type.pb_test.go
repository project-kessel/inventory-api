package v1beta2

import (
	"encoding/json"
	"google.golang.org/protobuf/proto"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRepresentationType_GetResourceType(t *testing.T) {
	rt := &RepresentationType{ResourceType: "host"}
	assert.Equal(t, "host", rt.GetResourceType())

	rtNil := (*RepresentationType)(nil)
	assert.Equal(t, "", rtNil.GetResourceType())
}

func TestRepresentationType_GetReporterType(t *testing.T) {
	reporter := "hbi"
	rt := &RepresentationType{ReporterType: &reporter}
	assert.Equal(t, "hbi", rt.GetReporterType())

	rtNoReporter := &RepresentationType{}
	assert.Equal(t, "", rtNoReporter.GetReporterType())

	rtNil := (*RepresentationType)(nil)
	assert.Equal(t, "", rtNil.GetReporterType())
}

func TestRepresentationType_Reset(t *testing.T) {
	reporter := "hbi"
	rt := &RepresentationType{
		ResourceType: "host",
		ReporterType: &reporter,
	}
	rt.Reset()
	assert.Equal(t, "", rt.GetResourceType())
	assert.Equal(t, "", rt.GetReporterType())
}

func TestRepresentationType_String(t *testing.T) {
	reporter := "hbi"
	rt := &RepresentationType{
		ResourceType: "host",
		ReporterType: &reporter,
	}
	s := rt.String()
	assert.NotEmpty(t, s)
	assert.Contains(t, s, "host")
	assert.Contains(t, s, "hbi")
}

func TestRepresentationType_ProtoMessage(t *testing.T) {
	var rt interface{} = &RepresentationType{}
	_, ok := rt.(proto.Message)
	assert.True(t, ok)
}

func TestRepresentationType_ProtoReflect(t *testing.T) {
	reporter := "hbi"
	rt := &RepresentationType{ResourceType: "host", ReporterType: &reporter}
	m := rt.ProtoReflect()
	assert.NotNil(t, m)
	assert.Equal(t, "host", m.Interface().(*RepresentationType).GetResourceType())
}

func TestRepresentationType_JSONRoundTrip(t *testing.T) {
	reporter := "hbi"
	rt := &RepresentationType{
		ResourceType: "host",
		ReporterType: &reporter,
	}
	b, err := json.Marshal(rt)
	assert.NoError(t, err)

	var out RepresentationType
	assert.NoError(t, json.Unmarshal(b, &out))
	assert.Equal(t, "host", out.GetResourceType())
	assert.Equal(t, "hbi", out.GetReporterType())
}

func TestRepresentationType_JSONRoundTrip_NoReporter(t *testing.T) {
	rt := &RepresentationType{
		ResourceType: "host",
	}
	b, err := json.Marshal(rt)
	assert.NoError(t, err)

	var out RepresentationType
	assert.NoError(t, json.Unmarshal(b, &out))
	assert.Equal(t, "host", out.GetResourceType())
	assert.Equal(t, "", out.GetReporterType())
}

func TestRepresentationType_AllNil(t *testing.T) {
	var rt *RepresentationType
	assert.Equal(t, "", rt.GetResourceType())
	assert.Equal(t, "", rt.GetReporterType())
}
