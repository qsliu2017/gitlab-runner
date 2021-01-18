// +build fips

package crypto

import (
	_ "crypto/tls/fipsonly"
	"fmt"
)

func init() {
	fmt.Println("FIPS crypto mode activated!")
}
