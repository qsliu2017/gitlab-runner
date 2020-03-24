package helpers

import (
	"fmt"
	"time"
)

const (
	traceSectionStart = "section_start:%v:%s\r" + ANSI_CLEAR
	traceSectionEnd   = "section_end:%v:%s\r" + ANSI_CLEAR
)

// RawLogger is a logger for sending non-formatted records to output.
type RawLogger interface {
	SendRawLog(args ...interface{})
}

// BuildSection is a wrapper around a build executable which allows for logs
// to be grouped and folded in the build output on Gitlab.
// If SkipMetrics is true, only the header will be shown.
type BuildSection struct {
	Name        string
	Header      string
	SkipMetrics bool
	Run         func() error
}

// Execute executes the Run function and outputs its logs in a foldable section in job output.
func (s *BuildSection) Execute(logger RawLogger) error {
	s.start(logger)
	defer s.end(logger)

	return s.Run()
}

func (s *BuildSection) start(logger RawLogger) {
	s.timestamp(traceSectionStart, logger)
	s.header(logger)
}

func (s *BuildSection) end(logger RawLogger) {
	s.timestamp(traceSectionEnd, logger)
}

func (s *BuildSection) header(logger RawLogger) {
	logger.SendRawLog(fmt.Sprintf("%s%s%s\n", ANSI_BOLD_CYAN, s.Header, ANSI_RESET))
}

func (s *BuildSection) timestamp(format string, logger RawLogger) {
	if s.SkipMetrics {
		return
	}

	sectionLine := fmt.Sprintf(format, nowUnixUTC(), s.Name)
	logger.SendRawLog(sectionLine)
}

func nowUnixUTC() int64 {
	return time.Now().UTC().Unix()
}
