package auth

import (
	"fmt"
	"reflect"
	"testing"

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
			Enabled: true,
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
	prefix := "consumer.retry-options"
	fs := pflag.NewFlagSet("", pflag.ContinueOnError)
	test.options.AddFlags(fs, prefix)

	structValues := reflect.ValueOf(*test.options)
	for i := 0; i < structValues.Type().NumField(); i++ {
		flagName := structValues.Type().Field(i).Tag.Get("mapstructure")
		assert.NotNil(t, fs.Lookup(fmt.Sprintf("%s.%s", prefix, flagName)))
	}
}
