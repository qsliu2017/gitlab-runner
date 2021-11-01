package buildtest

import (
	"bytes"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitlab.com/gitlab-org/gitlab-runner/common"
)

//nolint:funlen
func RunBuildWithCancel(t *testing.T, config *common.RunnerConfig, setup BuildSetupFn) {
	require.NotEmpty(t, config.Shell)

	cancelIncludeStages := []common.BuildStage{
		common.BuildStagePrepare,
		common.BuildStageGetSources,
	}
	cancelExcludeStages := []common.BuildStage{
		common.BuildStageRestoreCache,
		common.BuildStageDownloadArtifacts,
		common.BuildStageAfterScript,
		common.BuildStageArchiveOnSuccessCache,
		common.BuildStageArchiveOnFailureCache,
		common.BuildStageUploadOnFailureArtifacts,
		common.BuildStageUploadOnSuccessArtifacts,
	}

	tests := map[string]struct {
		onUserStep    func(*common.Build, common.JobTrace)
		includesStage []common.BuildStage
		excludesStage []common.BuildStage
		expectedErr   error
	}{
		"system interrupt": {
			onUserStep: func(build *common.Build, _ common.JobTrace) {
				build.SystemInterrupt <- os.Interrupt
			},
			includesStage: cancelIncludeStages,
			excludesStage: cancelExcludeStages,
			expectedErr:   &common.BuildError{FailureReason: common.RunnerSystemFailure},
		},
		"job is aborted": {
			onUserStep: func(_ *common.Build, trace common.JobTrace) {
				trace.Abort()
			},
			includesStage: cancelIncludeStages,
			excludesStage: cancelExcludeStages,
			expectedErr:   &common.BuildError{FailureReason: common.JobCanceled},
		},
		"job is canceling": {
			onUserStep: func(_ *common.Build, trace common.JobTrace) {
				trace.Cancel()
			},
			includesStage: cancelIncludeStages,
			excludesStage: cancelExcludeStages,
			expectedErr:   &common.BuildError{FailureReason: common.JobCanceled},
		},
	}

	for tn, tc := range tests {
		t.Run(tn, func(t *testing.T) {
			build := &common.Build{
				JobResponse:     common.GetRemoteBuildCancelScript(config.Shell),
				Runner:          config,
				SystemInterrupt: make(chan os.Signal, 1),
			}
			buf := new(bytes.Buffer)
			trace := NewTraceWriteLogger(&common.Trace{Writer: buf})
			trace.On([]byte("buildcancel: build script is executing"), func() {
				// At this point, the script has been running for some time and
				// we've detected that the script has output our expected text.
				// onUserStep cancels or aborts the build and we eventually see
				// that syscall.Kill is called to terminate the process. This
				// call returns without an error but is entirely ignored unless
				// this sleep is performed first (at least on macos). I've no
				// idea why this is the case.
				time.Sleep(time.Second)

				tc.onUserStep(build, trace)
			})
			if setup != nil {
				setup(build)
			}

			err := RunBuildWithTrace(t, build, trace)
			t.Log(buf.String())
			assert.ErrorIs(t, err, tc.expectedErr)

			assert.Contains(t, buf.String(), "buildcancel: build script exit trap executed")

			for _, stage := range tc.includesStage {
				assert.Contains(t, buf.String(), common.GetStageDescription(stage))
			}
			for _, stage := range tc.excludesStage {
				assert.NotContains(t, buf.String(), common.GetStageDescription(stage))
			}
		})
	}
}
