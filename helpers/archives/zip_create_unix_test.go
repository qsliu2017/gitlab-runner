// +build darwin dragonfly freebsd linux netbsd openbsd

package archives

import (
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
)

func createTestPipe(t *testing.T) string {
	err := syscall.Mkfifo("test_pipe", 0600)
	assert.NoError(t, err)
	return "test_pipe"
}
