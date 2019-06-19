package generic_exec

import (
	"time"

	"gitlab.com/gitlab-org/gitlab-runner/common"
)

type config struct {
	*common.GenericExecConfig
}

func (c *config) GetPrepareScriptTimeout() time.Duration {
	return getDuration(c.PrepareExecTimeout, defaultPrepareScriptTimeout)
}

func (c *config) GetCleanupScriptTimeout() time.Duration {
	return getDuration(c.CleanupExecTimeout, defaultCleanupScriptTimeout)
}

func (c *config) GetProcessKillTimeout() time.Duration {
	return getDuration(c.ExecKillTimeout, defaultProcessKillTimeout)
}

func (c *config) GetProcessKillGracePeriod() time.Duration {
	return getDuration(c.ExecKillGracePeriod, defaultProcessKillGracePeriod)
}

func getDuration(source *int, defaultValue time.Duration) time.Duration {
	if source == nil {
		return defaultValue
	}

	timeout := *source
	if timeout <= 0 {
		return defaultValue
	}

	return time.Duration(timeout) * time.Second
}
