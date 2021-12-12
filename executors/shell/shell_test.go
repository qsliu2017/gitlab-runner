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
	"gitlab.com/gitlab-org/gitlab-runner/helpers/process"
	"gitlab.com/gitlab-org/gitlab-runner/shells/shellstest"
)

type initCommandCallback func(options process.CommandOptions)

func TestExecutor_Run(t *testing.T) {
	var testErr = errors.New("test error")
	var exitErr = &exec.ExitError{}

	tests := map[string]struct {
		commanderAssertions     func(*process.MockCommander, chan time.Time)
		processKillerAssertions func(*process.MockKillWaiter, chan time.Time)
		cancelJob               bool
		useAdditionalEnv        bool
		expectedErr             error
	}{
		"canceled job uses new process termination": {
			commanderAssertions: func(mCmd *process.MockCommander, waitCalled chan time.Time) {
				mCmd.On("Start").Return(nil).Once()
				mCmd.On("Wait").Run(func(args mock.Arguments) {
					close(waitCalled)
				}).Return(nil).Once()
			},
			processKillerAssertions: func(mProcessKillWaiter *process.MockKillWaiter, waitCalled chan time.Time) {
				mProcessKillWaiter.
					On("KillAndWait", mock.Anything, mock.Anything).
					Return(nil).
					WaitUntil(waitCalled)
			},
			cancelJob:   true,
			expectedErr: nil,
		},
		"job uses custom environment variables": {
			commanderAssertions: func(mCmd *process.MockCommander, waitCalled chan time.Time) {
				mCmd.On("Start").Return(nil).Once()
				mCmd.On("Wait").Run(func(args mock.Arguments) {
					close(waitCalled)
				}).Return(nil).Once()
			},
			processKillerAssertions: func(mProcessKillWaiter *process.MockKillWaiter, waitCalled chan time.Time) {

			},
			cancelJob:        false,
			expectedErr:      nil,
			useAdditionalEnv: true,
		},
		"cmd fails to start": {
			commanderAssertions: func(mCmd *process.MockCommander, _ chan time.Time) {
				mCmd.On("Start").Return(testErr).Once()
			},
			processKillerAssertions: func(_ *process.MockKillWaiter, _ chan time.Time) {

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
			processKillerAssertions: func(mProcessKillWaiter *process.MockKillWaiter, waitCalled chan time.Time) {
				mProcessKillWaiter.
					On("KillAndWait", mock.Anything, mock.Anything).
					Return(nil).
					WaitUntil(waitCalled)
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
			processKillerAssertions: func(mProcessKillWaiter *process.MockKillWaiter, waitCalled chan time.Time) {
				mProcessKillWaiter.
					On("KillAndWait", mock.Anything, mock.Anything).
					Return(nil).
					WaitUntil(waitCalled)
			},
			cancelJob:   false,
			expectedErr: &common.BuildError{},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			shellstest.OnEachShell(t, func(t *testing.T, shell string) {
				cmdCallback := getCommanderCallback(t, tt.useAdditionalEnv)
				mProcessKillWaiter, mCmd, cleanup := setupProcessMocks(t, cmdCallback)
				defer cleanup()

				waitCalled := make(chan time.Time)
				tt.commanderAssertions(mCmd, waitCalled)
				tt.processKillerAssertions(mProcessKillWaiter, waitCalled)

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

				var env common.JobVariables
				if tt.useAdditionalEnv {
					env = common.JobVariables{
						{Key: "ABC", Value: "123"},
						{Key: "XYZ", Value: "987"},
					}
				}

				cmd := common.ExecutorCommand{
					Script:        "echo hello",
					Predefined:    false,
					Context:       ctx,
					AdditionalEnv: env,
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

func setupProcessMocks(
	t *testing.T,
	newCommandCallback initCommandCallback,
) (*process.MockKillWaiter, *process.MockCommander, func()) {
	mProcessKillWaiter := new(process.MockKillWaiter)
	defer mProcessKillWaiter.AssertExpectations(t)
	mCmd := new(process.MockCommander)
	defer mCmd.AssertExpectations(t)

	oldNewProcessKillWaiter := newProcessKillWaiter
	oldCmd := newCommander

	newProcessKillWaiter = func(
		logger process.Logger,
		gracefulKillTimeout time.Duration,
		forceKillTimeout time.Duration,
	) process.KillWaiter {
		return mProcessKillWaiter
	}

	newCommander = func(executable string, args []string, options process.CommandOptions) process.Commander {
		if newCommandCallback != nil {
			newCommandCallback(options)
		}

		return mCmd
	}

	return mProcessKillWaiter, mCmd, func() {
		newProcessKillWaiter = oldNewProcessKillWaiter
		newCommander = oldCmd
	}
}

func getCommanderCallback(t *testing.T, useAdditionalEnv bool) initCommandCallback {
	if useAdditionalEnv {
		return func(options process.CommandOptions) {
			assert.Contains(t, options.Env, "ABC=123")
			assert.Contains(t, options.Env, "XYZ=987")
		}
	}

	return func(options process.CommandOptions) {
		assert.NotContains(t, options.Env, "ABC=123")
		assert.NotContains(t, options.Env, "XYZ=987")
	}
}
