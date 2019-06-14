package generic_script

import (
	"time"

	"gitlab.com/gitlab-org/gitlab-runner/common"
)

type config struct {
	*common.GenericScriptConfig
}

func (c *config) GetPrepareScriptTimeout() time.Duration {
	return getDuration(c.PrepareScriptTimeout, defaultPrepareScriptTimeout)
}

func (c *config) GetCleanupScriptTimeout() time.Duration {
	return getDuration(c.CleanupScriptTimeout, defaultCleanupScriptTimeout)
}

func (c *config) GetProcessKillTimeout() time.Duration {
	return getDuration(c.ProcessKillTimeout, defaultProcessKillTimeout)
}

func (c *config) GetProcessKillGracePeriod() time.Duration {
	return getDuration(c.ProcessKillGracePeriod, defaultProcessKillGracePeriod)
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
