package shell

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/executors"
	"gitlab.com/gitlab-org/gitlab-runner/helpers"
)

func TestWindowsProcessTermination(t *testing.T) {
	if helpers.SkipIntegrationTests(t, "powershell") {
		t.Skip()
	}

	e := &executor{
		AbstractExecutor: executors.AbstractExecutor{
			Trace: &common.Trace{Writer: os.Stdout},
			BuildShell: &common.ShellConfiguration{
				Command:   "powershell",
				Arguments: []string{"-NoLogo", "-NonInteractive"},
			},
		},
	}

	timeout := 2 * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	start := time.Now()
	err := e.Run(common.ExecutorCommand{
		Script:     "Start-Process -NoNewWindow sleep 25",
		Stage:      common.BuildStageUserScript,
		Predefined: true,
		Context:    ctx,
	})

	assert.NoError(t, err)
	assert.WithinDuration(t, start.Add(timeout), time.Now(), timeout)
}
