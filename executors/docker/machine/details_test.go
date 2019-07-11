package machine

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"gitlab.com/gitlab-org/gitlab-runner/common"
)

func TestmachineDetailsUsed(t *testing.T) {
	d := machineDetails{}
	d.State = machineStateIdle
	assert.False(t, d.isUsed())
	d.State = machineStateAcquired
	assert.True(t, d.isUsed())
	d.State = machineStateCreating
	assert.True(t, d.isUsed())
	d.State = machineStateUsed
	assert.True(t, d.isUsed())
	d.State = machineStateRemoving
	assert.True(t, d.isUsed())
}

func TestmachineDetailsMatcher(t *testing.T) {
	config := &common.RunnerConfig{
		RunnerSettings: common.RunnerSettings{
			Machine: &common.DockerMachine{
				MachineName: "test-machine-%s",
			},
		},
	}

	d := machineDetails{Name: newMachineName(config)}
	assert.True(t, d.match("test-machine-%s"))
	assert.False(t, d.match("test-other-machine-%s"))
}
