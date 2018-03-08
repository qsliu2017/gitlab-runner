package formatter

import (
	"fmt"
	"io"

	"github.com/sirupsen/logrus"
)

type BuildLogger struct {
	log         io.Writer
	logIsStdout bool
	entry       *logrus.Entry
}

func (e *BuildLogger) SendRawLog(args ...interface{}) {
	if e.log != nil {
		fmt.Fprint(e.log, args...)
	}
}

func (e *BuildLogger) sendLog(logger func(args ...interface{}), logPrefix string, args ...interface{}) {
	if e.log != nil {
		logLine := ScrubSecrets(logPrefix + fmt.Sprintln(args...))
		e.SendRawLog(logLine)
		e.SendRawLog(ANSI_RESET)

		if e.logIsStdout {
			return
		}
	}

	if len(args) == 0 {
		return
	}

	logger(args...)
}

func (e *BuildLogger) Debugln(args ...interface{}) {
	if e.entry == nil {
		return
	}
	e.entry.Debugln(args...)
}

func (e *BuildLogger) Println(args ...interface{}) {
	if e.entry == nil {
		return
	}
	e.sendLog(e.entry.Debugln, ANSI_CLEAR, args...)
}

func (e *BuildLogger) Infoln(args ...interface{}) {
	if e.entry == nil {
		return
	}
	e.sendLog(e.entry.Println, ANSI_BOLD_GREEN, args...)
}

func (e *BuildLogger) Warningln(args ...interface{}) {
	if e.entry == nil {
		return
	}
	e.sendLog(e.entry.Warningln, ANSI_YELLOW+"WARNING: ", args...)
}

func (e *BuildLogger) SoftErrorln(args ...interface{}) {
	if e.entry == nil {
		return
	}
	e.sendLog(e.entry.Warningln, ANSI_BOLD_RED+"ERROR: ", args...)
}

func (e *BuildLogger) Errorln(args ...interface{}) {
	if e.entry == nil {
		return
	}
	e.sendLog(e.entry.Errorln, ANSI_BOLD_RED+"ERROR: ", args...)
}

func NewBuildLogger(log io.Writer, isStdout bool, entry *logrus.Entry) BuildLogger {
	return BuildLogger{
		log:         log,
		logIsStdout: isStdout,
		entry:       entry,
	}
}
