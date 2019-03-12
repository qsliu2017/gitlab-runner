package commands

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/servers/debug"
)

func TestProcessRunner_BuildLimit(t *testing.T) {
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
	mJobTrace.On("SetMasked", mock.Anything)
	mJobTrace.On("Success")
	mJobTrace.On("Fail", mock.Anything, mock.Anything)

	mNetwork := common.MockNetwork{}
	defer mNetwork.AssertExpectations(t)
	mNetwork.On("RequestJob", mock.Anything, mock.Anything).Return(&jobData, true)
	mNetwork.On("ProcessJob", mock.Anything, mock.Anything).Return(&mJobTrace, nil)

	var runningBuilds uint32
	e := common.MockExecutor{}
	defer e.AssertExpectations(t)
	e.On("Prepare", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	e.On("Cleanup").Maybe().Return()
	e.On("Shell").Return(&common.ShellScriptInfo{Shell: "script-shell"})
	e.On("Finish", mock.Anything).Return(nil).Maybe()
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

	common.RegisterExecutor("multi-runner-build-limit", &p)

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

	// Start 5 builds.
	wg := sync.WaitGroup{}
	wg.Add(5)
	for i := 0; i < 5; i++ {
		go func(i int) {
			defer wg.Done()

			cmd.processRunner(i, &cfg, runners)
		}(i)
	}

	// Wait until at least two builds have started.
	for atomic.LoadUint32(&runningBuilds) < 2 {
		time.Sleep(10 * time.Millisecond)
	}

	err := cmd.processRunner(6, &cfg, runners)
	assert.EqualError(t, err, "failed to request job, runner limit met")

	// Wait for all builds to finish.
	wg.Wait()
}

func TestDebugServerInitialization(t *testing.T) {
	server := new(debug.MockServer)
	defer server.AssertExpectations(t)

	oldDebugServerFactory := debugServerFactory
	defer func() {
		debugServerFactory = oldDebugServerFactory
	}()
	debugServerFactory = func() (debug.Server, error) {
		return server, nil
	}

	collectorsMatcher := mock.MatchedBy(func(collectors debug.CollectorsMap) bool {
		return len(collectors) > 0
	})

	wg := new(sync.WaitGroup)
	wg.Add(1)

	server.On("RegisterPrometheusCollectors", collectorsMatcher).
		Return(nil).
		Once()
	server.On("RegisterDebugEndpoint", "jobs/list", mock.AnythingOfType("http.HandlerFunc")).
		Return(nil).
		Once()
	server.On("Start", mock.Anything).
		Return(nil).
		Once().
		Run(func(args mock.Arguments) {
			wg.Done()
		})

	cmd := newCommand()
	cmd.ListenAddress = "127.0.0.1:12345"

	err := cmd.Start(nil)
	assert.NoError(t, err)

	wg.Wait()
}
