// +build !integration

package commands

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"gitlab.com/gitlab-org/gitlab-runner/helpers"
	testHelpers "gitlab.com/gitlab-org/gitlab-runner/helpers/test"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/log/test"
)

func TestProcessRunner_BuildLimit(t *testing.T) {
	hook, cleanup := test.NewHook()
	defer cleanup()

	logrus.SetLevel(logrus.DebugLevel)
	logrus.SetOutput(ioutil.Discard)

	cfg := common.RunnerConfig{
		Limit:              2,
		RequestConcurrency: 10,
		RunnerSettings: common.RunnerSettings{
			Executor: "multi-runner-build-limit",
		},
	}

	jobData := common.JobResponse{
		ID: 1,
		Steps: []common.Step{
			{
				Name:         "sleep",
				Script:       common.StepScript{"sleep 10"},
				Timeout:      15,
				When:         "",
				AllowFailure: false,
			},
		},
	}

	mJobTrace := common.MockJobTrace{}
	defer mJobTrace.AssertExpectations(t)
	mJobTrace.On("SetFailuresCollector", mock.Anything)
	mJobTrace.On("Write", mock.Anything).Return(0, nil)
	mJobTrace.On("IsStdout").Return(false)
	mJobTrace.On("SetCancelFunc", mock.Anything)
	mJobTrace.On("SetAbortFunc", mock.Anything)
	mJobTrace.On("SetMasked", mock.Anything)
	mJobTrace.On("Success")

	mNetwork := common.MockNetwork{}
	defer mNetwork.AssertExpectations(t)
	mNetwork.On("RequestJob", mock.Anything, mock.Anything, mock.Anything).Return(&jobData, true)
	mNetwork.On("ProcessJob", mock.Anything, mock.Anything).Return(&mJobTrace, nil)

	var runningBuilds uint32
	e := common.MockExecutor{}
	defer e.AssertExpectations(t)
	e.On("Prepare", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	e.On("Cleanup").Maybe()
	e.On("Shell").Return(&common.ShellScriptInfo{Shell: "script-shell"})
	e.On("Finish", mock.Anything).Maybe()
	e.On("Run", mock.Anything).Run(func(args mock.Arguments) {
		atomic.AddUint32(&runningBuilds, 1)

		// Simulate work to fill up build queue.
		time.Sleep(1 * time.Second)
	}).Return(nil)

	p := common.MockExecutorProvider{}
	defer p.AssertExpectations(t)
	p.On("Acquire", mock.Anything).Return(nil, nil)
	p.On("Release", mock.Anything, mock.Anything).Return(nil).Maybe()
	p.On("CanCreate").Return(true).Once()
	p.On("GetDefaultShell").Return("bash").Once()
	p.On("GetFeatures", mock.Anything).Return(nil)
	p.On("Create").Return(&e)

	common.RegisterExecutorProvider("multi-runner-build-limit", &p)

	cmd := RunCommand{
		network:      &mNetwork,
		buildsHelper: newBuildsHelper(),
		configOptionsWithListenAddress: configOptionsWithListenAddress{
			configOptions: configOptions{
				config: &common.Config{
					User: "git",
				},
			},
		},
	}

	runners := make(chan *common.RunnerConfig)

	// Start 2 builds.
	wg := sync.WaitGroup{}
	wg.Add(3)
	for i := 0; i < 3; i++ {
		go func(i int) {
			defer wg.Done()

			err := cmd.processRunner(i, &cfg, runners)
			assert.NoError(t, err)
		}(i)
	}

	// Wait until at least two builds have started.
	for atomic.LoadUint32(&runningBuilds) < 2 {
		time.Sleep(10 * time.Millisecond)
	}

	// Wait for all builds to finish.
	wg.Wait()

	limitMetCount := 0
	for _, entry := range hook.AllEntries() {
		if strings.Contains(entry.Message, "runner limit met") {
			limitMetCount++
		}
	}

	assert.Equal(t, 1, limitMetCount)
}

func TestRunCommand_doJobRequest(t *testing.T) {
	returnedJob := new(common.JobResponse)

	waitForContext := func(ctx context.Context) {
		<-ctx.Done()
	}

	tests := map[string]struct {
		requestJob             func(ctx context.Context)
		passSignal             func(c *RunCommand)
		expectedContextTimeout bool
	}{
		"requestJob returns immediately": {
			requestJob:             func(_ context.Context) {},
			passSignal:             func(_ *RunCommand) {},
			expectedContextTimeout: false,
		},
		"requestJob hangs indefinitely": {
			requestJob:             waitForContext,
			passSignal:             func(_ *RunCommand) {},
			expectedContextTimeout: true,
		},
		"requestJob interrupted by interrupt signal": {
			requestJob: waitForContext,
			passSignal: func(c *RunCommand) {
				c.runInterruptSignal <- os.Interrupt
			},
			expectedContextTimeout: false,
		},
		"runFinished signal is passed": {
			requestJob: waitForContext,
			passSignal: func(c *RunCommand) {
				close(c.runFinished)
			},
			expectedContextTimeout: false,
		},
	}

	for tn, tt := range tests {
		t.Run(tn, func(t *testing.T) {
			runner := new(common.RunnerConfig)

			network := new(common.MockNetwork)
			defer network.AssertExpectations(t)

			network.On("RequestJob", mock.Anything, *runner, mock.Anything).
				Run(func(args mock.Arguments) {
					ctx, ok := args.Get(0).(context.Context)
					require.True(t, ok)

					tt.requestJob(ctx)
				}).
				Return(returnedJob, true).
				Once()

			c := &RunCommand{
				network:            network,
				runInterruptSignal: make(chan os.Signal),
				runFinished:        make(chan bool),
			}

			ctx, cancelFn := context.WithTimeout(context.Background(), 1*time.Second)
			defer cancelFn()

			go tt.passSignal(c)

			job, _ := c.doJobRequest(ctx, runner, nil)

			assert.Equal(t, returnedJob, job)

			if tt.expectedContextTimeout {
				assert.ErrorIs(t, ctx.Err(), context.DeadlineExceeded)
				return
			}
			assert.NoError(t, ctx.Err())
		})
	}
}

func TestRunCommand_infiniteIncomingJobs(t *testing.T) {
	testHelpers.SkipIfGitLabCIWithMessage(t, "This job should be executed manually")

	configTOML := `
concurrent = 1000
check_interval = 10
# log_level = "debug"

[[runners]]
  name = "dummy-runner"
  limit = 1000
  url = "http://gitlab.example.com/"
  token = "dummy-token"
  request_concurrency = 10
  executor = "dummy-executor"
  shell = "dummy-shell"
`

	f, err := ioutil.TempFile("", "config.toml")
	require.NoError(t, err)

	defer func() {
		_ = f.Close()
		_ = os.Remove(f.Name())
	}()

	_, err = fmt.Fprint(f, configTOML)
	require.NoError(t, err)
	_ = f.Close()

	testJob := &common.JobResponse{
		ID: 0,
		JobInfo: common.JobInfo{
			Name:        "",
			Stage:       "",
			ProjectID:   0,
			ProjectName: "",
		},
		GitInfo: common.GitInfo{
			RepoURL:   "",
			Ref:       "",
			Sha:       "",
			BeforeSha: "",
			RefType:   "",
			Refspecs:  nil,
			Depth:     0,
		},
		Steps: common.Steps{
			{
				Name: "user_script",
				Script: common.StepScript{
					"echo nothing",
				},
				Timeout:      10,
				When:         "always",
				AllowFailure: false,
			},
		},
	}

	doneCh := make(chan struct{})
	done := false

	shell := new(common.MockShell)
	defer shell.AssertExpectations(t)

	shell.On("GetName").Return("dummy-shell")
	shell.On("IsDefault").Return(true).Maybe()
	shell.On("GenerateScript", mock.Anything, mock.Anything).Return("script", nil)

	common.RegisterShell(shell)

	trace := new(common.MockJobTrace)
	defer trace.AssertExpectations(t)

	trace.On("SetFailuresCollector", mock.Anything)
	trace.On("Write", mock.Anything).Return(0, nil)
	trace.On("IsStdout").Return(false)
	trace.On("Success").Maybe()
	trace.On("Fail", mock.Anything, mock.Anything).Maybe()
	trace.On("SetCancelFunc", mock.Anything)
	trace.On("SetAbortFunc", mock.Anything)
	trace.On("SetMasked", mock.Anything)

	network := new(common.MockNetwork)
	defer network.AssertExpectations(t)

	network.On("RequestJob", mock.Anything, mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			time.Sleep(20 * time.Millisecond)
		}).
		Return(testJob, true).
		Times(100)
	network.On("RequestJob", mock.Anything, mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			time.Sleep(20 * time.Millisecond)
			if !done {
				close(doneCh)
				done = true
			}
		}).
		Return(nil, true)
	network.On("ProcessJob", mock.Anything, mock.Anything).Return(trace, nil)

	executor := new(common.MockExecutor)
	defer executor.AssertExpectations(t)

	executor.On("GetCurrentStage").Return(common.ExecutorStageCreated).Maybe()
	executor.On("Prepare", mock.Anything).Return(nil)
	executor.On("Finish", mock.Anything)
	executor.On("Cleanup")
	executor.On("Shell").Return(&common.ShellScriptInfo{
		Shell:         shell.GetName(),
		RunnerCommand: "gitlab-runner",
	})
	executor.On("Run", mock.Anything).
		Run(func(_ mock.Arguments) {
			time.Sleep(2 * time.Second)
		}).
		Return(nil)

	executorProvider := new(common.MockExecutorProvider)
	defer executorProvider.AssertExpectations(t)

	executorProvider.On("GetDefaultShell").Return("bash")
	executorProvider.On("CanCreate").Return(true)
	executorProvider.On("GetFeatures", mock.Anything).Return(nil)
	executorProvider.On("Acquire", mock.Anything).Return(nil, nil)
	executorProvider.On("Release", mock.Anything, mock.Anything)
	executorProvider.On("Create").
		Run(func(_ mock.Arguments) {
			time.Sleep(250 * time.Millisecond)
		}).
		Return(executor)

	common.RegisterExecutorProvider("dummy-executor", executorProvider)

	wg := new(sync.WaitGroup)
	wg.Add(1)

	rc := newRunCommand()
	rc.ConfigFile = f.Name()
	rc.network = network

	go func() {
		defer wg.Done()
		defer helpers.MakeFatalToPanic()()

		assert.NotPanics(t, func() {
			rc.Execute(nil)
		})
	}()

	<-doneCh

	rc.stopSignals <- syscall.SIGQUIT

	wg.Wait()
}
