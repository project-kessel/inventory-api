package oidc

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
		options:         NewOptions(),
		expectedOptions: &Options{},
	}
	assert.Equal(t, test.expectedOptions, NewOptions())
}

func TestOptions_AddFlags(t *testing.T) {
	test := struct {
		options *Options
	}{
		options: NewOptions(),
	}
	prefix := "authn.oidc"
	fs := pflag.NewFlagSet("", pflag.ContinueOnError)
	test.options.AddFlags(fs, prefix)

	// GrpcEndpoints is configured via YAML, not CLI flags, so exclude it from flag check
	exclude := []string{"grpc-endpoints"}
	helpers.AllOptionsHaveFlags(t, prefix, fs, *test.options, exclude)
}
