package consistency

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
			ReadAfterWriteEnabled:   true,
			ReadAfterWriteAllowlist: []string{},
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
	prefix := "consistency"
	fs := pflag.NewFlagSet("", pflag.ContinueOnError)
	test.options.AddFlags(fs, prefix)

	// the below logic ensures that every possible option defined in the Options type
	// has a defined flag for that option; postgres and sqlite3 options are skipped in favor of testing
	// in their own packages
	structValues := reflect.ValueOf(*test.options)
	for i := 0; i < structValues.Type().NumField(); i++ {
		flagName := structValues.Type().Field(i).Tag.Get("mapstructure")
		assert.NotNil(t, fs.Lookup(fmt.Sprintf("%s.%s", prefix, flagName)))
	}
}
