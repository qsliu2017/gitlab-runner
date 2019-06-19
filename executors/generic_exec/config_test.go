package generic_exec

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"gitlab.com/gitlab-org/gitlab-runner/common"
)

type getDurationTestCase struct {
	source        *int
	defaultValue  time.Duration
	expectedValue time.Duration
}

func testGetDuration(t *testing.T, defaultValue time.Duration, assert func(*testing.T, getDurationTestCase)) {
	tests := map[string]getDurationTestCase{
		"source undefined": {
			defaultValue:  defaultValue,
			expectedValue: defaultValue,
		},
		"source value lower than zero": {
			source:        func() *int { i := -10; return &i }(),
			defaultValue:  defaultValue,
			expectedValue: defaultValue,
		},
		"source value greater than zero": {
			source:        func() *int { i := 10; return &i }(),
			defaultValue:  defaultValue,
			expectedValue: time.Duration(10) * time.Second,
		},
	}

	for testName, tt := range tests {
		t.Run(testName, func(t *testing.T) {
			assert(t, tt)
		})
	}
}

func TestConfig_GetPrepareScriptTimeout(t *testing.T) {
	testGetDuration(t, defaultPrepareScriptTimeout, func(t *testing.T, tt getDurationTestCase) {
		c := &config{
			GenericExecConfig: &common.GenericExecConfig{
				PrepareExecTimeout: tt.source,
			},
		}

		assert.Equal(t, tt.expectedValue, c.GetPrepareScriptTimeout())
	})
}

func TestConfig_GetCleanupScriptTimeout(t *testing.T) {
	testGetDuration(t, defaultCleanupScriptTimeout, func(t *testing.T, tt getDurationTestCase) {
		c := &config{
			GenericExecConfig: &common.GenericExecConfig{
				CleanupExecTimeout: tt.source,
			},
		}

		assert.Equal(t, tt.expectedValue, c.GetCleanupScriptTimeout())
	})
}

func TestConfig_GetProcessKillTimeout(t *testing.T) {
	testGetDuration(t, defaultProcessKillTimeout, func(t *testing.T, tt getDurationTestCase) {
		c := &config{
			GenericExecConfig: &common.GenericExecConfig{
				ExecKillTimeout: tt.source,
			},
		}

		assert.Equal(t, tt.expectedValue, c.GetProcessKillTimeout())
	})
}

func TestConfig_GetProcessKillGracePeriod(t *testing.T) {
	testGetDuration(t, defaultProcessKillGracePeriod, func(t *testing.T, tt getDurationTestCase) {
		c := &config{
			GenericExecConfig: &common.GenericExecConfig{
				ExecKillGracePeriod: tt.source,
			},
		}

		assert.Equal(t, tt.expectedValue, c.GetProcessKillGracePeriod())
	})
}
