package shells

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitlab.com/gitlab-org/gitlab-runner/common"
)

func TestBash_CommandShellEscapes(t *testing.T) {
	writer := &BashWriter{}
	writer.Command("foo", "x&(y)")

	assert.Equal(t, `$'foo' "x&(y)"`+"\n", writer.String())
}

func TestBash_IfCmdShellEscapes(t *testing.T) {
	writer := &BashWriter{}
	writer.IfCmd("foo", "x&(y)")

	assert.Equal(t, `if $'foo' "x&(y)" >/dev/null 2>/dev/null; then`+"\n", writer.String())
}

const (
	fileVariableTestCase      string = "file variable"
	directoryVariableTestCase string = "directory variable"
	normalVariableTestCase    string = "normal variable"
)

type variableTestCase struct {
	variable     common.JobVariable
	assertOutput func(t *testing.T, output string)
}

func getVariableTestCases() map[string]*variableTestCase {
	return map[string]*variableTestCase{
		fileVariableTestCase: {
			variable: common.JobVariable{Key: "KEY", Value: "VALUE", File: true},
		},
		directoryVariableTestCase: {
			variable: common.JobVariable{Key: "KEY", Value: "VALUE", Directory: true},
		},
		normalVariableTestCase: {
			variable: common.JobVariable{Key: "KEY", Value: "VALUE"},
		},
	}
}

func testVariable(t *testing.T, variables map[string]*variableTestCase, writerFactory func() ShellWriter) {
	for tn, tt := range variables {
		t.Run(tn, func(t *testing.T) {
			writer := writerFactory()
			writer.Variable(tt.variable)

			require.NotNil(t, tt.assertOutput, "Must define assertOutput function in the test case")
			tt.assertOutput(t, writer.(fmt.Stringer).String())
		})
	}
}

func TestBashWriter_Variable(t *testing.T) {
	variableTestCases := getVariableTestCases()

	variableTestCases[fileVariableTestCase].assertOutput = func(t *testing.T, output string) {
		assert.Contains(t, output, `mkdir -p "$PWD/builds/test"`)
		assert.Contains(t, output, `echo -n VALUE > "$PWD/builds/test/KEY"`)
		assert.Contains(t, output, `export KEY="$PWD/builds/test/KEY"`)
	}

	variableTestCases[directoryVariableTestCase].assertOutput = func(t *testing.T, output string) {
		assert.Contains(t, output, `mkdir -p "$PWD/builds/test/KEY"`)
		assert.Contains(t, output, `export KEY="$PWD/builds/test/KEY"`)
	}

	variableTestCases[normalVariableTestCase].assertOutput = func(t *testing.T, output string) {
		assert.Contains(t, output, `export KEY=VALUE`)
	}

	testVariable(t, variableTestCases, func() ShellWriter {
		return &BashWriter{
			TemporaryPath: "builds/test",
		}
	})
}
