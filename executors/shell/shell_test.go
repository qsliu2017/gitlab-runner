// +build !integration

package shell

import (
	"context"
	"errors"
	"os/exec"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/executors"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/featureflags"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/process"
	"gitlab.com/gitlab-org/gitlab-runner/shells/shellstest"
)

func TestExecutor_Run(t *testing.T) {
	var testErr = errors.New("test error")
	var exitErr = &exec.ExitError{}

	tests := map[string]struct {
		commanderAssertions func(*process.MockCommander, chan time.Time)
		cancelJob           bool
		expectedErr         error
	}{
		"canceled job uses new process termination": {
			commanderAssertions: func(mCmd *process.MockCommander, waitCalled chan time.Time) {
				mCmd.On("Start").Return(nil).Once()
				mCmd.On("Wait").Run(func(args mock.Arguments) {
					close(waitCalled)
				}).Return(nil).Once()
			},
			cancelJob:   true,
			expectedErr: nil,
		},
		"cmd fails to start": {
			commanderAssertions: func(mCmd *process.MockCommander, _ chan time.Time) {
				mCmd.On("Start").Return(testErr).Once()
			},
			expectedErr: testErr,
		},
		"wait returns error": {
			commanderAssertions: func(mCmd *process.MockCommander, waitCalled chan time.Time) {
				mCmd.On("Start").Return(nil).Once()
				mCmd.On("Wait").Run(func(args mock.Arguments) {
					close(waitCalled)
				}).Return(testErr).Once()
			},
			cancelJob:   false,
			expectedErr: testErr,
		},
		"wait returns exit error": {
			commanderAssertions: func(mCmd *process.MockCommander, waitCalled chan time.Time) {
				mCmd.On("Start").Return(nil).Once()
				mCmd.On("Wait").Run(func(args mock.Arguments) {
					close(waitCalled)
				}).Return(exitErr).Once()
			},
			cancelJob:   false,
			expectedErr: &common.BuildError{},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			shellstest.OnEachShell(t, func(t *testing.T, shell string) {
				mCmd, cleanup := setupProcessMocks(t)
				defer cleanup()

				waitCalled := make(chan time.Time)
				tt.commanderAssertions(mCmd, waitCalled)

				executor := executor{
					AbstractExecutor: executors.AbstractExecutor{
						Build: &common.Build{
							JobResponse: common.JobResponse{},
							Runner:      &common.RunnerConfig{},
						},
						BuildShell: &common.ShellConfiguration{
							Command: shell,
						},
					},
				}

				ctx, cancelJob := context.WithCancel(context.Background())
				defer cancelJob()

				cmd := common.ExecutorCommand{
					Script:     "echo hello",
					Predefined: false,
					Context:    ctx,
				}

				if tt.cancelJob {
					cancelJob()
				}

				err := executor.Run(cmd)
				assert.ErrorIs(t, err, tt.expectedErr)
			})
		})
	}
}

func setupProcessMocks(t *testing.T) (*process.MockCommander, func()) {
	mCmd := new(process.MockCommander)
	defer mCmd.AssertExpectations(t)

	oldCmd := newCommander

	newCommander = func(context.Context, string, []string, process.CommandOptions) process.Commander {
		return mCmd
	}

	return mCmd, func() {
		newCommander = oldCmd
	}
}

// TODO: Remove in 14.0 https://gitlab.com/gitlab-org/gitlab-runner/issues/6413
func TestProcessTermination_Legacy(t *testing.T) {
	shellstest.OnEachShell(t, func(t *testing.T, shell string) {
		// cmd is deprecated and exec.Cmd#Wait does not propagate the correct
		// error on batch so skip this test. batch is being deprecated in 13.0
		// as well https://gitlab.com/gitlab-org/gitlab-runner/issues/6099
		if shell == "cmd" {
			return
		}

		executor := executor{
			AbstractExecutor: executors.AbstractExecutor{
				Build: &common.Build{
					JobResponse: common.JobResponse{
						Variables: common.JobVariables{
							common.JobVariable{Key: featureflags.ShellExecutorUseLegacyProcessKill, Value: "true"},
						},
					},
					Runner: &common.RunnerConfig{},
				},
				BuildShell: &common.ShellConfiguration{
					Command: shell,
				},
			},
		}

		ctx, cancelJob := context.WithCancel(context.Background())

		cmd := common.ExecutorCommand{
			Script:     "echo hello",
			Predefined: false,
			Context:    ctx,
		}

		cancelJob()

		err := executor.Run(cmd)
		var buildErr *common.BuildError
		assert.ErrorAs(t, err, &buildErr)
	})
}
