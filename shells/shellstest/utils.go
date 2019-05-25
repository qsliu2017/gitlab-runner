package shellstest

import (
	"testing"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/helpers"
	"gitlab.com/gitlab-org/gitlab-runner/shells"
)

type shellWriterFactory func() shells.ShellWriter

func OnEachShell(t *testing.T, f func(t *testing.T, shell common.Shell)) {
	for _, shell := range common.GetShells() {
		t.Run(shell, func(t *testing.T) {
			if helpers.SkipIntegrationTests(t, shell) {
				t.Skip()
			}

			f(t, common.GetShell(shell))
		})
	}
}
