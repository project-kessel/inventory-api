package helpers

import (
	"fmt"
	"reflect"
	"slices"
	"testing"

	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
)

// AllOptionsHaveFlags is a test helper that verifies all struct fields have corresponding flags.
// It uses reflection to check that each field (identified by mapstructure tag) has an associated
// flag in the provided flag set, except for those listed in skippedFlags.
func AllOptionsHaveFlags(t *testing.T, prefix string, flags *pflag.FlagSet, options interface{}, skippedFlags []string) {
	structValues := reflect.ValueOf(options)
	for i := 0; i < structValues.Type().NumField(); i++ {
		flagName := structValues.Type().Field(i).Tag.Get("mapstructure")
		if slices.Contains(skippedFlags, flagName) {
			continue
		} else {
			assert.NotNil(t, flags.Lookup(fmt.Sprintf("%s.%s", prefix, flagName)))
		}
	}
}
