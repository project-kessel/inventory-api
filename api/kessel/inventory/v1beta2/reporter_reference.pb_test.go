package v1beta2_test

import (
	"testing"

	"buf.build/go/protovalidate"
	"github.com/project-kessel/inventory-api/api/kessel/inventory/v1beta2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func TestReporterReference_ValidInstanceIdHostname(t *testing.T) {
	instID := "redhat.com"
	r := &v1beta2.ReporterReference{
		Type:       "hbi",
		InstanceId: &instID,
	}
	assert.Equal(t, "hbi", r.GetType())
	assert.Equal(t, "redhat.com", r.GetInstanceId())
}

func TestReporterReference_ValidInstanceIdHostname_NoDot(t *testing.T) {
	instID := "redhat"
	r := &v1beta2.ReporterReference{
		Type:       "hbi",
		InstanceId: &instID,
	}
	assert.Equal(t, "hbi", r.GetType())
	assert.Equal(t, "redhat", r.GetInstanceId())
}

func TestReporterReference_InvalidInstanceIdHostname_Comma(t *testing.T) {
	instID := "redhat,com"
	r := &v1beta2.ReporterReference{
		Type:       "hbi",
		InstanceId: &instID,
	}
	validator, err := protovalidate.New()
	require.NoError(t, err)
	err = validator.Validate(r)
	assert.Error(t, err)
}

func TestReporterReference_InvalidInstanceIdHostname_Hashtag(t *testing.T) {
	instID := "redhat#com"
	r := &v1beta2.ReporterReference{
		Type:       "hbi",
		InstanceId: &instID,
	}
	validator, err := protovalidate.New()
	require.NoError(t, err)
	err = validator.Validate(r)
	assert.Error(t, err)
}

func TestReporterReference_InvalidInstanceIdHostname_AtSign(t *testing.T) {
	instID := "redhat@com"
	r := &v1beta2.ReporterReference{
		Type:       "hbi",
		InstanceId: &instID,
	}
	validator, err := protovalidate.New()
	require.NoError(t, err)
	err = validator.Validate(r)
	assert.Error(t, err)
}

func TestReporterReference_InvalidInstanceIdUUID_SingleDigit(t *testing.T) {
	instID := "3"
	r := &v1beta2.ReporterReference{
		Type:       "hbi",
		InstanceId: &instID,
	}
	validator, err := protovalidate.New()
	require.NoError(t, err)
	err = validator.Validate(r)
	assert.Error(t, err)
}

func TestReporterReference_InvalidInstanceIdUUID_MultipleDigits(t *testing.T) {
	instID := "3-3-3-3-3"
	r := &v1beta2.ReporterReference{
		Type:       "hbi",
		InstanceId: &instID,
	}
	validator, err := protovalidate.New()
	require.NoError(t, err)
	err = validator.Validate(r)
	assert.Error(t, err)
}

func TestReporterReference_InvalidInstanceIdUUID_TooLong(t *testing.T) {
	instID := "3c4e2382-26c1-11f0-8e5c-ce0194e9e144-11212"
	r := &v1beta2.ReporterReference{
		Type:       "hbi",
		InstanceId: &instID,
	}
	validator, err := protovalidate.New()
	require.NoError(t, err)
	err = validator.Validate(r)
	assert.Error(t, err)
}

func TestReporterReference_InvalidInstanceIdUUID_TooShort(t *testing.T) {
	instID := "3c4e2382-26c1-11f0-8e5c-ce01944"
	r := &v1beta2.ReporterReference{
		Type:       "hbi",
		InstanceId: &instID,
	}
	validator, err := protovalidate.New()
	require.NoError(t, err)
	err = validator.Validate(r)
	assert.Error(t, err)
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
