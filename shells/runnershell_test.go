package shells

import (
	"testing"

	"github.com/stretchr/testify/require"
	"gitlab.com/gitlab-org/gitlab-runner/common"
)

func BenchmarkRunnerShellShack(b *testing.B) {
	resp, err := common.GetRemoteSuccessfulBuild()
	if err != nil {
		b.Fatal(err)
	}

	info := common.ShellScriptInfo{
		Build: &common.Build{
			JobResponse: resp,
			Runner:      &common.RunnerConfig{},
		},
	}

	for n := 0; n < b.N; n++ {
		stack := &RunnerShell{Shell: &BashShell{}}

		script, err := stack.GenerateScript(common.BuildStageGetSources, info)
		if err != nil {
			b.Fatal(err)
		}

		_ = script
	}
}

func TestRunnerShellGenerateScript(t *testing.T) {
	stack := &RunnerShell{}

	resp, err := common.GetRemoteSuccessfulBuild()
	require.NoError(t, err)

	_, err = stack.GenerateScript(common.BuildStageGetSources, common.ShellScriptInfo{
		Build: &common.Build{
			JobResponse: resp,
			Runner:      &common.RunnerConfig{},
		},
	})
	require.NoError(t, err)
}
