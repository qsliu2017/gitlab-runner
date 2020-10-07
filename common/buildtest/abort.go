package buildtest

import (
	"bytes"
	"errors"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitlab.com/gitlab-org/gitlab-runner/common"
)

//nolint:funlen
func RunBuildWithCancel(t *testing.T, config *common.RunnerConfig, setup BuildSetupFn) {
	abortIncludeStages := []common.BuildStage{
		common.BuildStagePrepare,
		common.BuildStageGetSources,
	}
	abortExcludeStages := []common.BuildStage{
		common.BuildStageRestoreCache,
		common.BuildStageDownloadArtifacts,
		common.BuildStageAfterScript,
		common.BuildStageArchiveOnSuccessCache,
		common.BuildStageArchiveOnFailureCache,
		common.BuildStageUploadOnFailureArtifacts,
		common.BuildStageUploadOnSuccessArtifacts,
	}

	tests := map[string]struct {
		stage         string
		onStage       func(*common.Build, common.JobTrace)
		includesStage []common.BuildStage
		excludesStage []common.BuildStage
		expectedErr   error
	}{
		"system interrupt at user stage": {
			stage: "step_",
			onStage: func(build *common.Build, _ common.JobTrace) {
				build.SystemInterrupt <- os.Interrupt
			},
			includesStage: abortIncludeStages,
			excludesStage: abortExcludeStages,
			expectedErr:   &common.BuildError{FailureReason: common.RunnerSystemFailure},
		},
		"job is aborted at user stage": {
			stage: "step_",
			onStage: func(_ *common.Build, trace common.JobTrace) {
				trace.Abort()
			},
			includesStage: abortIncludeStages,
			excludesStage: abortExcludeStages,
			expectedErr:   &common.BuildError{FailureReason: common.JobAborted},
		},
		"job is canceling at prepare stage": {
			stage: string(common.BuildStagePrepare),
			onStage: func(_ *common.Build, trace common.JobTrace) {
				trace.Cancel()
			},
			includesStage: []common.BuildStage{
				common.BuildStagePrepare,
			},
			excludesStage: []common.BuildStage{
				common.BuildStageGetSources,
				common.BuildStageAfterScript,
				common.BuildStageRestoreCache,
				common.BuildStageDownloadArtifacts,
				common.BuildStageArchiveOnSuccessCache,
				common.BuildStageUploadOnFailureArtifacts,
				common.BuildStageUploadOnFailureArtifacts,
				common.BuildStageUploadOnSuccessArtifacts,
			},
			expectedErr: &common.BuildError{FailureReason: common.JobCanceled},
		},
		"job is canceling at user stage": {
			stage: "step_",
			onStage: func(_ *common.Build, trace common.JobTrace) {
				trace.Cancel()
			},
			includesStage: []common.BuildStage{
				common.BuildStagePrepare,
				common.BuildStageGetSources,
				common.BuildStageAfterScript,
			},
			excludesStage: []common.BuildStage{
				common.BuildStageRestoreCache,
				common.BuildStageDownloadArtifacts,
				common.BuildStageUploadOnFailureArtifacts,
				common.BuildStageUploadOnSuccessArtifacts,
			},
			expectedErr: &common.BuildError{FailureReason: common.JobCanceled},
		},
	}

	resp, err := common.GetRemoteLongRunningBuildWithAfterScript()
	require.NoError(t, err)

	for tn, tc := range tests {
		t.Run(tn, func(t *testing.T) {
			build := &common.Build{
				JobResponse:     resp,
				Runner:          config,
				SystemInterrupt: make(chan os.Signal, 1),
			}
			buf := new(bytes.Buffer)
			trace := &common.Trace{Writer: buf}

			defer OnStage(build, tc.stage, func() {
				tc.onStage(build, trace)
			})()
			if setup != nil {
				setup(build)
			}

			err := RunBuildWithTrace(t, build, trace)
			t.Log(buf.String())
			//nolint:lll
			assert.True(t, errors.Is(err, tc.expectedErr), "expected: %[1]T (%[1]v), got: %[2]T (%[2]v)", tc.expectedErr, err)

			for _, stage := range tc.includesStage {
				assert.Contains(t, buf.String(), common.GetStageDescription(stage))
			}
			for _, stage := range tc.excludesStage {
				assert.NotContains(t, buf.String(), common.GetStageDescription(stage))
			}
		})
	}
}
