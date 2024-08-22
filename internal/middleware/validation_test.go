package middleware

import (
	"context"
	"testing"

	"github.com/bufbuild/protovalidate-go"
	"github.com/project-kessel/inventory-api/api/kessel/inventory/v1beta1"

	"github.com/stretchr/testify/assert"
)

func TestValidation_ValidRequest(t *testing.T) {
	t.Parallel()

	validator, err := protovalidate.New()
	assert.NoError(t, err)

	m := Validation(validator)

	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return "success", nil
	}

	resp, err := m(handler)(context.Background(), &v1beta1.CreateRhelHostRequest{
		RhelHost: &v1beta1.RhelHost{
			Metadata: &v1beta1.Metadata{},
			ReporterData: &v1beta1.ReporterData{
				ReporterType:    1,
				LocalResourceId: "1",
			},
		},
	})
	assert.NoError(t, err)
	assert.Equal(t, "success", resp)
}

func TestValidation_InvalidRequest(t *testing.T) {
	t.Parallel()

	validator, err := protovalidate.New()
	assert.NoError(t, err)

	m := Validation(validator)

	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return nil, nil
	}

	resp, err := m(handler)(context.Background(), &v1beta1.CreateRhelHostRequest{})
	assert.Error(t, err)
	assert.Equal(t, resp, nil)
}
