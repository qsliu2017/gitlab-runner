package shells

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPowershell_CommandShellEscapes(t *testing.T) {
	writer := &PsWriter{}
	writer.Command("foo", "x&(y)")

	assert.Equal(t, "& \"foo\" \"x&(y)\"\r\nif(!$?) { Exit $LASTEXITCODE }\r\n\r\n", writer.String())
}

func TestPowershell_IfCmdShellEscapes(t *testing.T) {
	writer := &PsWriter{}
	writer.IfCmd("foo", "x&(y)")

	assert.Equal(t, "& \"foo\" \"x&(y)\" 2>$null\r\nif($?) {\r\n", writer.String())
}

func TestPowershell_UseTryCatch(t *testing.T) {
	tests := []struct {
		errorActionPreference bool
		expectedScript        string
	}{
		{
			errorActionPreference: true,
			expectedScript:        "Try {\r\n  & \"echo\" \"hello\" 2>$null\r\n  echo hello2\r\n} Catch {\r\n  echo hello3\r\n}\r\nTry {\r\n  & \"echo\" \"output\"\r\n  echo output2\r\n} Catch {\r\n  echo output3\r\n}\r\n",
		},
		{
			errorActionPreference: false,
			expectedScript:        "& \"echo\" \"hello\" 2>$null\r\nif($?) {\r\n  echo hello2\r\n} else {\r\n  echo hello3\r\n}\r\n  & \"echo\" \"output\"\r\n  if($?) {\r\n  echo output2\r\n} else {\r\n  echo output3\r\n}\r\n",
		},
	}

	for _, test := range tests {
		t.Run(fmt.Sprintf("errorActionPreference: %t", test.errorActionPreference), func(t *testing.T) {
			w := &PsWriter{
				useTryCatch: test.errorActionPreference,
			}

			w.IfCmd("echo", "hello")
			w.Line("echo hello2")
			w.Else()
			w.Line("echo hello3")
			w.EndIf()

			w.IfCmdWithOutput("echo", "output")
			w.Line("echo output2")
			w.Else()
			w.Line("echo output3")
			w.EndIf()

			assert.Equal(t, test.expectedScript, w.String())
		})
	}
}
