package shells

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPowershell_LineBreaks(t *testing.T) {
	testCases := map[string]struct {
		shell                   string
		eol                     string
		expectedEdition         string
		expectedErrorPreference string
	}{
		"Windows newline on Desktop": {
			shell:           "powershell",
			eol:             "\r\n",
			expectedEdition: "Desktop",
		},
		"Windows newline on Core": {
			shell:                   "pwsh",
			eol:                     "\r\n",
			expectedEdition:         "Core",
			expectedErrorPreference: `$ErrorActionPreference = "Stop"` + "\r\n\r\n",
		},
		"Linux newline on Core": {
			shell:                   "pwsh",
			eol:                     "\n",
			expectedEdition:         "Core",
			expectedErrorPreference: `$ErrorActionPreference = "Stop"` + "\n\n",
		},
	}
	for tn, tc := range testCases {
		t.Run(tn, func(t *testing.T) {
			eol := tc.eol
			writer := &PsWriter{Shell: tc.shell, EOL: eol}
			writer.Command("foo", "")

			assert.Equal(
				t,
				"\xef\xbb\xbf"+
					`#requires -PSEdition `+tc.expectedEdition+eol+
					eol+
					tc.expectedErrorPreference+
					`& "foo" ""`+eol+"if(!$?) { Exit &{if($LASTEXITCODE) {$LASTEXITCODE} else {1}} }"+eol+
					eol+
					eol,
				writer.Finish(false))
		})
	}
}

func TestPowershell_CommandShellEscapes(t *testing.T) {
	writer := &PsWriter{Shell: "pwsh", EOL: "\r\n"}
	writer.Command("foo", "x&(y)")

	assert.Equal(t, "& \"foo\" \"x&(y)\"\r\nif(!$?) { Exit &{if($LASTEXITCODE) {$LASTEXITCODE} else {1}} }\r\n\r\n", writer.String())
}

func TestPowershell_IfCmdShellEscapes(t *testing.T) {
	writer := &PsWriter{Shell: "pwsh", EOL: "\r\n"}
	writer.IfCmd("foo", "x&(y)")

	assert.Equal(t, "Set-Variable -Name cmdErr -Value $false\r\nTry {\r\n  & \"foo\" \"x&(y)\" 2>$null\r\n  if(!$?) { throw &{if($LASTEXITCODE) {$LASTEXITCODE} else {1}} }\r\n} Catch {\r\n  Set-Variable -Name cmdErr -Value $true\r\n}\r\nif(!$cmdErr) {\r\n", writer.String())
}
