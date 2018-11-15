package shells

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPwsh_CommandShellEscapes(t *testing.T) {
	writer := &PwshWriter{}
	writer.Command("foo", "x&(y)")

	assert.Equal(t, "& \"foo\" \"x&(y)\"\r\nif(!$?) { Exit $LASTEXITCODE }\r\n\r\n", writer.String())
}

func TestPwsh_IfCmdShellEscapes(t *testing.T) {
	writer := &PwshWriter{}
	writer.IfCmd("foo", "x&(y)")

	assert.Equal(t, "& \"foo\" \"x&(y)\" 2>$null\r\nif($?) {\r\n", writer.String())
}
