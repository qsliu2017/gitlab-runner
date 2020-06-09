package main

import (
	"fmt"
)

func list() {
	fmt.Printf("\n Distribution versions supported with DEB/RPM packages:\n\n")

	for _, distribution := range distributions.Distributions {
		for _, versionInfo := range distribution.Versions {
			fmt.Printf("%20s %-20s %-20s %s\n", distribution.Name, versionInfo.Version, versionInfo.EOL, versionInfo.IDs)
		}
	}
}
