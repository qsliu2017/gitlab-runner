package errors

import (
	"fmt"
)

type UnsupportedOSTypeError struct {
	detectedOSType string
}

func (e *UnsupportedOSTypeError) Error() string {
	return fmt.Sprintf("unsupported OSType %q", e.detectedOSType)
}

func NewUnsupportedOSTypeError(osType string) *UnsupportedOSTypeError {
	return &UnsupportedOSTypeError{
		detectedOSType: osType,
	}
}
