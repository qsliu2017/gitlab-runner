package buildtest

import (
	"bytes"
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/featureflags"
)

func RunBuildWithSections(t *testing.T, config *common.RunnerConfig) {
	successfulBuild, err := common.GetSuccessfulBuild()
	require.NoError(t, err)

	successfulBuild.Features.TraceSections = true

	build := &common.Build{
		JobResponse: successfulBuild,
		Runner:      config,
	}

	build.Variables = common.JobVariables{
		{
			Key:   featureflags.ScriptSections,
			Value: "true",
		},
	}

	buf := new(bytes.Buffer)
	trace := &common.Trace{Writer: buf}
	err = RunBuildWithTrace(t, build, trace)
	assert.NoError(t, err)
	//nolint:lll
	// section_start:1627911560:section_27e4a11ba6450738\r\x1b[0K\x1b[32;1m$ echo Hello World\x1b[0;m\nHello World\n\x1b[0Ksection_end:1627911560:section_27e4a11ba6450738
	assert.Regexp(t, regexp.MustCompile("(?s)section_start:[0-9]+:section_[0-9a-f]+.*echo Hello World.*section_end:[0-9]+:section_[0-9a-f]"), buf.String())
}
