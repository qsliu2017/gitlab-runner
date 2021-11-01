package buildtest

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"gitlab.com/gitlab-org/gitlab-runner/common"
)

const testTimeout = 30 * time.Minute

type traceLogger struct {
	common.JobTrace

	buf bytes.Buffer
	on  []*traceLoggerOn
	mu  sync.Mutex
}

type traceLoggerOn struct {
	data []byte
	fn   func()
	done bool
}

func NewTraceWriteLogger(trace common.JobTrace) *traceLogger {
	return &traceLogger{
		JobTrace: trace,
	}
}

func (t *traceLogger) On(data []byte, fn func()) {
	t.on = append(t.on, &traceLoggerOn{data, fn, false})
}

func (t *traceLogger) Write(p []byte) (n int, err error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	n, err = t.JobTrace.Write(p)

	_, _ = t.buf.Write(p[:n])

	for _, on := range t.on {
		if !on.done && bytes.Contains(t.buf.Bytes(), on.data) {
			on.done = true
			on.fn()
		}
	}

	if idx := bytes.LastIndex(t.buf.Bytes(), []byte("\n")); idx > -1 {
		t.buf.Truncate(idx)
	}

	return n, err
}

type BuildSetupFn func(build *common.Build)

func RunBuildReturningOutput(t *testing.T, build *common.Build) (string, error) {
	buf := new(bytes.Buffer)
	err := RunBuildWithTrace(t, build, &common.Trace{Writer: buf})
	output := buf.String()
	t.Log(output)

	return output, err
}

func RunBuildWithTrace(t *testing.T, build *common.Build, trace common.JobTrace) error {
	return RunBuildWithOptions(t, build, trace, &common.Config{})
}

func RunBuildWithOptions(t *testing.T, build *common.Build, trace common.JobTrace, config *common.Config) error {
	timeoutTimer := time.AfterFunc(testTimeout, func() {
		t.Log("Timed out")
		t.FailNow()
	})
	defer timeoutTimer.Stop()

	return build.Run(config, trace)
}

func RunBuild(t *testing.T, build *common.Build) error {
	err := RunBuildWithTrace(t, build, &common.Trace{Writer: os.Stdout})

	return err
}

// OnStage executes the provided function when the provided stage is entered.
func OnStage(build *common.Build, stage string, fn func()) func() {
	exit := make(chan struct{})

	go func() {
		for {
			select {
			case <-exit:
				return

			case <-time.After(200 * time.Millisecond):
				currentStage := string(build.CurrentStage())
				if strings.HasPrefix(currentStage, stage) {
					fn()
					return
				}
			}
		}
	}()

	return func() {
		close(exit)
	}
}

// OnUserStage executes the provided function when the CurrentStage() enters
// a non-predefined stage.
func OnUserStage(build *common.Build, fn func()) func() {
	return OnStage(build, "step_", fn)
}

func SetBuildFeatureFlag(build *common.Build, flag string, value bool) {
	for _, v := range build.Variables {
		if v.Key == flag {
			v.Value = fmt.Sprint(value)
			return
		}
	}

	build.Variables = append(build.Variables, common.JobVariable{
		Key:   flag,
		Value: fmt.Sprint(value),
	})
}

type baseJobGetter func() (common.JobResponse, error)

// getJobResponseWithCommands is a wrapper that will decorate a JobResponse getter
// like common.GetRemoteSuccessfulBuild with a custom commands list
func getJobResponseWithCommands(t *testing.T, baseJobGetter baseJobGetter, commands ...string) common.JobResponse {
	jobResponse, err := baseJobGetter()
	require.NoError(t, err)

	jobResponse.Steps[0].Script = commands

	return jobResponse
}

// WithFeatureFlags runs a subtest for the on/off value for each flag provided,
// and allows a build object as part of the test to be decorated with the
// feature flag variable.
func WithEachFeatureFlag(t *testing.T, f func(t *testing.T, setup BuildSetupFn), flags ...string) {
	if len(flags) == 0 {
		t.Log("WithEachFeatureFlag: no feature flags provided. Running inner test with no feature flags.")
		f(t, func(build *common.Build) {})
		return
	}

	for _, flag := range flags {
		for _, value := range []bool{false, true} {
			t.Run(fmt.Sprintf("%v=%v", flag, value), func(t *testing.T) {
				f(t, func(build *common.Build) {
					SetBuildFeatureFlag(build, flag, value)
				})
			})
		}
	}
}
