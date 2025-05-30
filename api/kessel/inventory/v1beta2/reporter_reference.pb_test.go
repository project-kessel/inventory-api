package v1beta2_test

import (
	"testing"

	"github.com/project-kessel/inventory-api/api/kessel/inventory/v1beta2"
	"github.com/stretchr/testify/assert"
)

func TestReporterReference_BasicFields(t *testing.T) {
	instID := "hbi"
	r := &v1beta2.ReporterReference{
		Type:       "host",
		InstanceId: &instID,
	}

	assert.Equal(t, "host", r.GetType())
	assert.Equal(t, "hbi", r.GetInstanceId())
}

func TestReporterReference_NilInstanceId(t *testing.T) {
	r := &v1beta2.ReporterReference{
		Type: "k8s_policy",
	}
	assert.Equal(t, "k8s_policy", r.GetType())
	assert.Equal(t, "", r.GetInstanceId())
}

func TestReporterReference_ResetAndFields(t *testing.T) {
	instID := "hello"
	r := &v1beta2.ReporterReference{
		Type:       "test",
		InstanceId: &instID,
	}

	assert.Equal(t, "test", r.GetType())
	assert.Equal(t, "hello", r.GetInstanceId())

	r.Reset()
	assert.Equal(t, "", r.Type)
	assert.Nil(t, r.InstanceId)
}

func TestReporterReference_Methods(t *testing.T) {
	instID := "xyz"
	r := &v1beta2.ReporterReference{
		Type:       "reporter",
		InstanceId: &instID,
	}

	// String method
	s := r.String()
	assert.Contains(t, s, "reporter")

	// ProtoMessage, ProtoReflect, Descriptor: just call them for coverage
	r.ProtoMessage()
	_ = r.ProtoReflect()
	_, _ = r.Descriptor()
}
