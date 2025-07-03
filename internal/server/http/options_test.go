package http

import (
	"testing"

	"github.com/project-kessel/inventory-api/internal/helpers"
	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
)

func TestNewOptions(t *testing.T) {
	test := struct {
		options         *Options
		expectedOptions *Options
	}{
		options: NewOptions(),
		expectedOptions: &Options{
			Addr:    "0.0.0.0:8000",
			Timeout: 300,
			CertOpt: 3,
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
	prefix := "server.http"
	fs := pflag.NewFlagSet("", pflag.ContinueOnError)
	test.options.AddFlags(fs, prefix)

	helpers.AllOptionsHaveFlags(t, prefix, fs, *test.options, nil)
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
				Addr:    "0.0.0.0:8000",
				Timeout: 300,
				CertOpt: 3,
			},
			expectError: false,
		},
		{
			name: "invalid timeout setting",
			options: &Options{
				Addr:    "0.0.0.0:8000",
				Timeout: -1,
				CertOpt: 3,
			},
			expectError: true,
		},
		{
			name: "missing cert file when key file is provided",
			options: &Options{
				Addr:           "0.0.0.0:8000",
				Timeout:        300,
				CertOpt:        3,
				PrivateKeyFile: "/fake/path/key.pem",
			},
			expectError: true,
		},
		{
			name: "missing key file when cert file is provided",
			options: &Options{
				Addr:            "0.0.0.0:8000",
				Timeout:         300,
				CertOpt:         3,
				ServingCertFile: "/fake/path/cert.pem",
			},
			expectError: true,
		},
		{
			name: "both key file and cert file are provided",
			options: &Options{
				Addr:            "0.0.0.0:8000",
				Timeout:         300,
				CertOpt:         3,
				ServingCertFile: "/fake/path/cert.pem",
				PrivateKeyFile:  "/fake/path/key.pem",
			},
			expectError: false,
		},
		{
			name: "certOpt set to < 0",
			options: &Options{
				Addr:    "0.0.0.0:8000",
				Timeout: 300,
				CertOpt: -1,
			},
			expectError: true,
		},
		{
			name: "certOpt set to > 4",
			options: &Options{
				Addr:    "0.0.0.0:8000",
				Timeout: 300,
				CertOpt: 5,
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
