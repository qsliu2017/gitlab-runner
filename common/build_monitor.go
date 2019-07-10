package common

import (
	"fmt"

	"github.com/sirupsen/logrus"

	"gitlab.com/gitlab-org/gitlab-runner/helpers"
	url_helpers "gitlab.com/gitlab-org/gitlab-runner/helpers/url"
)

type BuildMonitor struct {
	metrics JobMetrics[]
}

func (e *BuildLogger) SendRawLog(args ...interface{}) {
	if e.log != nil {
		fmt.Fprint(e.log, args...)
	}
}

func (e *BuildLogger) sendLog(logger func(args ...interface{}), logPrefix string, args ...interface{}) {
	if e.log != nil {
		logLine := url_helpers.ScrubSecrets(logPrefix + fmt.Sprintln(args...))
		e.SendRawLog(logLine)
		e.SendRawLog(helpers.ANSI_RESET)

		if e.log.IsStdout() {
			return
		}
	}

	if len(args) == 0 {
		return
	}

	logger(args...)
}

func NewBuildLogger(log JobTrace, entry *logrus.Entry) BuildLogger {
	return BuildLogger{
		log:   log,
		entry: entry,
	}
}
