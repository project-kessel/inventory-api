package server

import (
	"os"
	"testing"

	"github.com/project-kessel/inventory-api/internal/helpers"
	"github.com/project-kessel/inventory-api/internal/server/grpc"
	"github.com/project-kessel/inventory-api/internal/server/http"
	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
)

func TestNewOptions(t *testing.T) {
	id, _ := os.Hostname()
	test := struct {
		options         *Options
		expectedOptions *Options
	}{
		options: NewOptions(),
		expectedOptions: &Options{
			Id:        id,
			Name:      "kessel-inventory-api",
			PublicUrl: "http://localhost:8000",

			GrpcOptions: grpc.NewOptions(),
			HttpOptions: http.NewOptions(),
		},
	}
	assert.Equal(t, test.expectedOptions, NewOptions())
}

func TestOptions_AddFlags(t *testing.T) {
	test := struct {
		options *Options
	}{
		options: NewOptions(),
	}
	prefix := "server"
	fs := pflag.NewFlagSet("", pflag.ContinueOnError)
	test.options.AddFlags(fs, prefix)

	// the below logic ensures that every possible option defined in the Options type
	// has a defined flag for that option; grpc and http are skipped in favor of testing
	// in their own package
	helpers.AllOptionsHaveFlags(t, prefix, fs, *test.options, []string{"grpc", "http"})
}

func TestOptions_Validate(t *testing.T) {
	tests := []struct {
		name        string
		options     *Options
		expectError bool
	}{
		{
			name: "valid options",
			options: &Options{
				GrpcOptions: grpc.NewOptions(),
				HttpOptions: http.NewOptions(),
			},
			expectError: false,
		},
		{
			name: "grpc validation fails therefore validate fails",
			options: &Options{
				GrpcOptions: &grpc.Options{
					Timeout: -1,
				},
				HttpOptions: http.NewOptions(),
			},
			expectError: true,
		},
		{
			name: "http validation fails therefore validate fails",
			options: &Options{
				GrpcOptions: grpc.NewOptions(),
				HttpOptions: &http.Options{
					Timeout: -1,
				},
			},
			expectError: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			errs := test.options.Validate()
			if test.expectError {
				assert.NotNil(t, errs)
			} else {
				assert.Nil(t, errs)
			}
		})
	}
}
