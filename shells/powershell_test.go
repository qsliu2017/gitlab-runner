package shells

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPowershell_CommandShellEscapes(t *testing.T) {
	writer := &PsWriter{}
	writer.Command("foo", "x&(y)")

	assert.Equal(t, "& \"foo\" \"x&(y)\"\r\nif(!$?) { Exit &{if($LASTEXITCODE) {$LASTEXITCODE} else {1}} }\r\n\r\n", writer.String())
}

func TestPowershell_IfCmdShellEscapes(t *testing.T) {
	writer := &PsWriter{}
	writer.IfCmd("foo", "x&(y)")

	assert.Equal(t, "Set-Variable -Name cmdErr -Value $false\r\nTry {\r\n  & \"foo\" \"x&(y)\" 2>$null\r\n  if(!$?) { throw &{if($LASTEXITCODE) {$LASTEXITCODE} else {1}} }\r\n} Catch {\r\n  Set-Variable -Name cmdErr -Value $true\r\n}\r\nif(!$cmdErr) {\r\n", writer.String())
}

func TestPsWriter_Variable(t *testing.T) {
	variableTestCases := getVariableTestCases()

	variableTestCases[fileVariableTestCase].assertOutput = func(t *testing.T, output string) {
		assert.Contains(t, output, `$CurrentDirectory = (Resolve-Path .\).Path`)
		assert.Contains(t, output, `New-Item -ItemType directory -Force -Path "$CurrentDirectory\builds\test" | out-null`)
		assert.Contains(t, output, `Set-Content "$CurrentDirectory\builds\test\KEY" -Value "VALUE" -Encoding UTF8 -Force`)
		assert.Contains(t, output, `$KEY="$CurrentDirectory\builds\test\KEY"`)
		assert.Contains(t, output, `$env:KEY=$KEY`)
	}

	variableTestCases[directoryVariableTestCase].assertOutput = func(t *testing.T, output string) {
		assert.Contains(t, output, `$CurrentDirectory = (Resolve-Path .\).Path`)
		assert.Contains(t, output, `New-Item -ItemType directory -Force -Path "$CurrentDirectory\builds\test\KEY" | out-null`)
		assert.Contains(t, output, `$KEY="$CurrentDirectory\builds\test\KEY"`)
		assert.Contains(t, output, `$env:KEY=$KEY`)
	}

	variableTestCases[normalVariableTestCase].assertOutput = func(t *testing.T, output string) {
		assert.Contains(t, output, `$KEY="VALUE"`)
		assert.Contains(t, output, `$env:KEY=$KEY`)
	}

	testVariable(t, variableTestCases, func() ShellWriter {
		return &PsWriter{
			TemporaryPath: "builds\\test",
		}
	})
}
