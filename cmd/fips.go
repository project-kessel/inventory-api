//go:build fips_enabled

package cmd

import (
	_ "crypto/tls/fipsonly"
	"fmt"
)

func init() {
	fmt.Println("***** Starting with FIPS crypto enabled *****")
}
