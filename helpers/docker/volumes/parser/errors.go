package parser

import (
	"fmt"
)

type InvalidVolumeSpecError struct {
	spec string
}

func (e *InvalidVolumeSpecError) Error() string {
	return fmt.Sprintf("invalid volume specification: %q", e.spec)
}

func newInvalidVolumeSpecErr(spec string) error {
	return &InvalidVolumeSpecError{
		spec: spec,
	}
}

// TODO: join with the one from helperimage package and
//       move to a common place
type unsupportedOSTypeError struct {
	detectedOSType string
}

func (e *unsupportedOSTypeError) Error() string {
	return fmt.Sprintf("unsupported OSType %q", e.detectedOSType)
}

func newUnsupportedOSTypeError(osType string) *unsupportedOSTypeError {
	return &unsupportedOSTypeError{
		detectedOSType: osType,
	}
}
