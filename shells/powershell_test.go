package shells

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"gitlab.com/gitlab-org/gitlab-runner/common"
)

func TestPowershell_LineBreaks(t *testing.T) {
	testCases := map[string]struct {
		shell                   string
		eol                     string
		expectedEdition         string
		expectedErrorPreference string
	}{
		"Windows newline on Desktop": {
			shell:                   "powershell",
			eol:                     "\r\n",
			expectedEdition:         "Desktop",
			expectedErrorPreference: "",
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

			expectedOutput := `#requires -PSEdition ` + tc.expectedEdition + eol +
				eol +
				tc.expectedErrorPreference +
				`& "foo" ""` + eol + "if(!$?) { Exit &{if($LASTEXITCODE) {$LASTEXITCODE} else {1}} }" + eol +
				eol +
				eol
			assert.Equal(t, expectedOutput, writer.Finish(false))
		})
	}
}

func TestPowershell_CommandShellEscapes(t *testing.T) {
	writer := &PsWriter{Shell: "powershell", EOL: "\r\n"}
	writer.Command("foo", "x&(y)")

	assert.Equal(
		t,
		"& \"foo\" \"x&(y)\"\r\nif(!$?) { Exit &{if($LASTEXITCODE) {$LASTEXITCODE} else {1}} }\r\n\r\n",
		writer.String(),
	)
}

func TestPowershell_IfCmdShellEscapes(t *testing.T) {
	writer := &PsWriter{Shell: "powershell", EOL: "\r\n"}
	writer.IfCmd("foo", "x&(y)")

	//nolint:lll
	assert.Equal(t, "Set-Variable -Name cmdErr -Value $false\r\nTry {\r\n  & \"foo\" \"x&(y)\" 2>$null\r\n  if(!$?) { throw &{if($LASTEXITCODE) {$LASTEXITCODE} else {1}} }\r\n} Catch {\r\n  Set-Variable -Name cmdErr -Value $true\r\n}\r\nif(!$cmdErr) {\r\n", writer.String())
}

func TestPowershell_MkTmpDirOnUNCShare(t *testing.T) {
	writer := &PsWriter{TemporaryPath: `\\unc-server\share`, EOL: "\n"}
	writer.MkTmpDir("tmp")

	assert.Equal(
		t,
		`New-Item -ItemType directory -Force -Path "\\unc-server\share\tmp" | out-null`+writer.EOL,
		writer.String(),
	)
}

func TestPowershell_Encodings(t *testing.T) {
	testCases := map[string]struct {
		Encoding    Encoding
		shell       string
		script      string
		expectedBOM bool
	}{
		"Powershell default": {
			shell:       "powershell",
			script:      "echo 'hello world'",
			expectedBOM: false,
		},
		"Powershell UTF8": {
			Encoding:    UTF8,
			shell:       "powershell",
			script:      "echo 'hello world'",
			expectedBOM: false,
		},
		"Powershell UTF8 with special character": {
			Encoding:    UTF8,
			shell:       "powershell",
			script:      "echo 'hello 世界'",
			expectedBOM: true,
		},
		"Powershell UTF8BOM": {
			Encoding:    UTF8BOM,
			shell:       "powershell",
			script:      "echo 'hello world'",
			expectedBOM: true,
		},
		"Powershell UTF8BOM with special character": {
			Encoding:    UTF8BOM,
			shell:       "powershell",
			script:      "echo 'hello 世界'",
			expectedBOM: true,
		},
		"Powershell UTF8NoBOM": {
			Encoding:    UTF8NoBOM,
			shell:       "powershell",
			script:      "echo 'hello world'",
			expectedBOM: false,
		},
		"Powershell UTF8NoBOM with special character": {
			Encoding:    UTF8NoBOM,
			shell:       "powershell",
			script:      "echo 'hello 世界'",
			expectedBOM: false,
		},

		"Powershell Core UTF8": {
			Encoding:    UTF8,
			shell:       "pwsh",
			script:      "echo 'hello world'",
			expectedBOM: false,
		},
		"Powershell Core UTF8 with special character": {
			Encoding:    UTF8,
			shell:       "pwsh",
			script:      "echo 'hello 世界'",
			expectedBOM: false,
		},
		"Powershell Core UTF8BOM": {
			Encoding:    UTF8BOM,
			shell:       "pwsh",
			script:      "echo 'hello world'",
			expectedBOM: true,
		},
		"Powershell Core UTF8BOM with special character": {
			Encoding:    UTF8BOM,
			shell:       "pwsh",
			script:      "echo 'hello 世界'",
			expectedBOM: true,
		},
		"Powershell Core UTF8NoBOM": {
			Encoding:    UTF8NoBOM,
			shell:       "pwsh",
			script:      "echo 'hello world'",
			expectedBOM: false,
		},
		"Powershell Core UTF8NoBOM with special character": {
			Encoding:    UTF8NoBOM,
			shell:       "pwsh",
			script:      "echo 'hello 世界'",
			expectedBOM: false,
		},
	}

	for tn, tc := range testCases {
		t.Run(tn, func(t *testing.T) {
			shell := &PowerShell{Shell: tc.shell}
			script, err := shell.GenerateScript(
				common.BuildStageAfterScript,
				common.ShellScriptInfo{
					Shell:         tc.shell,
					Type:          common.NormalShell,
					RunnerCommand: "gitlab-runner",
					Build: &common.Build{
						Runner: &common.RunnerConfig{},
						JobResponse: common.JobResponse{
							Steps: common.Steps{
								{
									Name:   common.StepNameAfterScript,
									Script: common.StepScript([]string{tc.script}),
								},
							},
							Variables: common.JobVariables{
								{
									Key:   "POWERSHELL_ENCODING",
									Value: string(tc.Encoding),
								},
							},
						},
					},
				},
			)
			assert.NoError(t, err)

			hasBOM := strings.HasPrefix(script, "\xef\xbb\xbf")
			if tc.expectedBOM {
				assert.True(t, hasBOM)
			} else {
				assert.False(t, hasBOM)
			}
		})
	}
}
