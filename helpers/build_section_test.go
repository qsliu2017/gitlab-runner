package helpers

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

type testBuffer struct {
	bytes.Buffer
	Error error
}

func (b *testBuffer) SendRawLog(args ...interface{}) {
	if b.Error != nil {
		return
	}

	_, b.Error = b.WriteString(fmt.Sprintln(args...))
}

func TestBuildSection(t *testing.T) {
	for num, tc := range []struct {
		name          string
		sectionHeader string
		skipMetrics   bool
		error         error
	}{
		{"Success", "Header", false, nil},
		{"Failure", "Header", false, fmt.Errorf("Failing test")},
		{"SkipMetricsSuccess", "Header", true, nil},
		{"SkipMetricsFailure", "Header", true, fmt.Errorf("Failing test")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			logger := new(testBuffer)

			section := BuildSection{
				Name:        tc.name,
				Header:      tc.sectionHeader,
				SkipMetrics: tc.skipMetrics,
				Run:         func() error { return tc.error },
			}
			section.Execute(logger)

			output := logger.String()
			assert.Nil(t, logger.Error, "case %d: Error: %s", num, logger.Error)
			for _, str := range []string{"section_start:", "section_end:", tc.name} {
				if tc.skipMetrics {
					assert.NotContains(t, output, str)
				} else {
					assert.Contains(t, output, tc.sectionHeader)
					assert.Contains(t, output, str)
				}
			}
		})
	}
}
