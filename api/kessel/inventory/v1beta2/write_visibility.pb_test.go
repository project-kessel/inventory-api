package v1beta2

import (
	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/reflect/protoreflect"
	"testing"
)

func TestWriteVisibility_Enum(t *testing.T) {
	ev := WriteVisibility_WRITE_VISIBILITY_UNSPECIFIED
	ptr := ev.Enum()
	assert.NotNil(t, ptr)
	assert.Equal(t, ev, *ptr)
}

func TestWriteVisibility_String(t *testing.T) {
	assert.Equal(t, "WRITE_VISIBILITY_UNSPECIFIED", WriteVisibility_WRITE_VISIBILITY_UNSPECIFIED.String())
	assert.Equal(t, "MINIMIZE_LATENCY", WriteVisibility_MINIMIZE_LATENCY.String())
	assert.Equal(t, "IMMEDIATE", WriteVisibility_IMMEDIATE.String())
}

func TestWriteVisibility_Number(t *testing.T) {
	assert.Equal(t, protoreflect.EnumNumber(0), WriteVisibility_WRITE_VISIBILITY_UNSPECIFIED.Number())
	assert.Equal(t, protoreflect.EnumNumber(1), WriteVisibility_MINIMIZE_LATENCY.Number())
	assert.Equal(t, protoreflect.EnumNumber(2), WriteVisibility_IMMEDIATE.Number())
}

func TestWriteVisibility_Descriptor(t *testing.T) {
	// Descriptor() and Type() should return a non-nil value
	assert.NotNil(t, WriteVisibility_WRITE_VISIBILITY_UNSPECIFIED.Descriptor())
	assert.NotNil(t, WriteVisibility_WRITE_VISIBILITY_UNSPECIFIED.Type())
}

func TestWriteVisibility_ValueMaps(t *testing.T) {
	assert.Equal(t, int32(0), WriteVisibility_value["WRITE_VISIBILITY_UNSPECIFIED"])
	assert.Equal(t, int32(1), WriteVisibility_value["MINIMIZE_LATENCY"])
	assert.Equal(t, int32(2), WriteVisibility_value["IMMEDIATE"])

	assert.Equal(t, "WRITE_VISIBILITY_UNSPECIFIED", WriteVisibility_name[0])
	assert.Equal(t, "MINIMIZE_LATENCY", WriteVisibility_name[1])
	assert.Equal(t, "IMMEDIATE", WriteVisibility_name[2])
}

func TestWriteVisibility_EnumDescriptor(t *testing.T) {
	b, idx := WriteVisibility_WRITE_VISIBILITY_UNSPECIFIED.EnumDescriptor()
	assert.NotNil(t, b)
	assert.NotNil(t, idx)
}
