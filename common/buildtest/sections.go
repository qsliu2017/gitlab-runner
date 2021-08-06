package buildtest

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/featureflags"
)

func RunBuildWithSections(t *testing.T, build *common.Build) {
	build.Features.TraceSections = true
	build.Variables = common.JobVariables{
		{
			Key:   featureflags.ScriptSections,
			Value: "true",
		},
	}

	buf := new(bytes.Buffer)
	trace := &common.Trace{Writer: buf}
	assert.NoError(t, RunBuildWithTrace(t, build, trace))
	//nolint:lll
	// section_start:1627911560:section_27e4a11ba6450738\r\x1b[0K\x1b[32;1m$ echo Hello World\x1b[0;m\nHello World\n\x1b[0Ksection_end:1627911560:section_27e4a11ba6450738

	fmt.Printf("THIS IS THE JOB LOG: \n\n%v\n", buf.String())
	//	assert.Regexp(t, regexp.MustCompile("(?s)section_start:[0-9]+:section_[0-9a-f]+.*Hello World.*section_end:[0-9]+:section_[0-9a-f]"), buf.String())
}
