package deprecate

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

type testLogger struct {
	buf bytes.Buffer
}

func (l *testLogger) Warningln(args ...interface{}) {
	fmt.Fprintln(&l.buf, args...)
}

func (l *testLogger) Debugln(args ...interface{}) {
	fmt.Fprintln(&l.buf, args...)
}

func TestDeprecationLog(t *testing.T) {
	logger := &testLogger{}

	Warningln(logger, "2227", "14.0 will replace the 'build_script' with 'step_script'")

	assert.Contains(t, logger.buf.String(),
		"deprecation[2227](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/2227): "+
			"14.0 will replace the 'build_script' with 'step_script'")
}
func TestDeprecationSuppressedLog(t *testing.T) {
	logger := &testLogger{}

	suppressEnv([]string{
		"SUPPRESS_DEPRECATION_1000",
		"SUPPRESS_DEPRECATION_1001=1",
		"SUPPRESS_DEPRECATION_1002=true",
		"SUPPRESS_DEPRECATION_1003=false",
		"SUPPRESS_DEPRECATION_1004=0",
	})

	Warningln(logger, "1000", "failure")
	Warningln(logger, "1001", "failure")
	Debugln(logger, "1002", "failure")
	Debugln(logger, "1003", "success-1003")
	Warningln(logger, "1004", "success-1004")
	Warningln(logger, "1005", "success-1005")

	output := logger.buf.String()

	assert.NotContains(t, output, "failure")
	assert.Contains(t, output, "success-1003")
	assert.Contains(t, output, "success-1004")
	assert.Contains(t, output, "success-1005")
}
