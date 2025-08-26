package v1beta2_test

import (
	"testing"

	"github.com/project-kessel/inventory-api/api/kessel/inventory/v1beta2"
	"github.com/stretchr/testify/assert"
)

func TestReporterReference_BasicFields(t *testing.T) {
	instID := "3c4e2382-26c1-11f0-8e5c-ce0194e9e144"
	r := &v1beta2.ReporterReference{
		Type:       "hbi",
		InstanceId: &instID,
	}

	assert.Equal(t, "hbi", r.GetType())
	assert.Equal(t, "3c4e2382-26c1-11f0-8e5c-ce0194e9e144", r.GetInstanceId())
}

func TestReporterReference_NilInstanceId(t *testing.T) {
	r := &v1beta2.ReporterReference{
		Type: "hbi",
	}
	assert.Equal(t, "hbi", r.GetType())
	assert.Equal(t, "", r.GetInstanceId())
}

func TestReporterReference_ResetAndFields(t *testing.T) {
	instID := "3c4e2382-26c1-11f0-8e5c-ce0194e9e144"
	r := &v1beta2.ReporterReference{
		Type:       "hbi",
		InstanceId: &instID,
	}

	assert.Equal(t, "hbi", r.GetType())
	assert.Equal(t, "3c4e2382-26c1-11f0-8e5c-ce0194e9e144", r.GetInstanceId())

	r.Reset()
	assert.Equal(t, "", r.Type)
	assert.Nil(t, r.InstanceId)
}

func TestReporterReference_Methods(t *testing.T) {
	instID := "3c4e2382-26c1-11f0-8e5c-ce0194e9e144"
	r := &v1beta2.ReporterReference{
		Type:       "hbi",
		InstanceId: &instID,
	}

	// String method
	s := r.String()
	assert.Contains(t, s, "hbi")

	// ProtoMessage, ProtoReflect, Descriptor: just call them for coverage
	r.ProtoMessage()
	_ = r.ProtoReflect()
	_, _ = r.Descriptor()
}
