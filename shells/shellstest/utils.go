package shellstest

import (
	"testing"

	"github.com/stretchr/testify/require"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/helpers"
	"gitlab.com/gitlab-org/gitlab-runner/shells"
)

type shellWriterFactory func() shells.ShellWriter

func OnEachShell(t *testing.T, f func(t *testing.T, shell string)) {
	for _, shell := range common.GetShells() {
		t.Run(shell, func(t *testing.T) {
			if helpers.SkipIntegrationTests(t, shell) {
				t.Skip()
			}

			f(t, shell)
		})

	}
}

func OnEachShellWithWriter(t *testing.T, f func(t *testing.T, shell string, writer shells.ShellWriter)) {
	writers := map[string]shellWriterFactory{
		"bash": func() shells.ShellWriter {
			return &shells.BashWriter{Shell: "bash"}
		},
		// TODO: How to fix that?
		// "cmd": func() shells.ShellWriter {
		// 	return &shells.CmdWriter{}
		// },
		// "powershell": func() shells.ShellWriter {
		// 	return &shells.PsWriter{}
		// },
	}

	OnEachShell(t, func(t *testing.T, shell string) {
		writer := writers[shell]
		require.NotEmpty(t, writer, "Missing factory for %s", shell)

		f(t, shell, writer())
	})
}
