package v1beta1

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestMetadataValid(t *testing.T) {
	meta := Metadata{}
	err := meta.ValidateAll()

	assert.NoError(t, err)
}

func TestMetadataEmptyLabels(t *testing.T) {
	meta := Metadata{
		Labels: []*ResourceLabel{},
	}

	err := meta.ValidateAll()
	assert.NoError(t, err)
}

func TestMetadataInvalidLabels(t *testing.T) {
	meta := Metadata{
		Labels: []*ResourceLabel{
			{},
		},
	}

	err := meta.ValidateAll()
	assert.ErrorContains(t, err, "invalid Metadata.Labels[0]")
}
