package log

import (
	"bufio"
	"io"
	"strings"

	"github.com/sirupsen/logrus"
)

type Logger interface {
	logrus.FieldLogger

	NewStdoutWriter(command string, reader io.Reader)
	NewStderrWriter(command string, reader io.Reader)
}

type logger struct {
	*logrus.Logger
}

func New() Logger {
	logrusLogger := logrus.New()
	logrusLogger.SetFormatter(new(logrus.TextFormatter))
	logrusLogger.SetLevel(logrus.DebugLevel)

	l := &logger{
		Logger: logrusLogger,
	}

	return l
}

func (l *logger) NewStdoutWriter(command string, reader io.Reader) {
	go newLogWriter(l, l.WithField("command", command).Info, reader).watch()
}

func (l *logger) NewStderrWriter(command string, reader io.Reader) {
	go newLogWriter(l, l.WithField("command", command).Error, reader).watch()
}

type logWriter struct {
	logger Logger
	output func(args ...interface{})
	reader *bufio.Reader
}

func newLogWriter(l Logger, o func(args ...interface{}), r io.Reader) *logWriter {
	return &logWriter{
		logger: l,
		output: o,
		reader: bufio.NewReader(r),
	}
}

func (lw *logWriter) watch() {
	for {
		line, err := lw.reader.ReadString('\n')
		if err == nil || err == io.EOF {
			lw.write(line)
			if err == io.EOF {
				return
			}

			continue
		}

		if !strings.Contains(err.Error(), "bad file descriptor") {
			lw.logger.WithError(err).Error("Problem while reading command output")
		}
		return
	}
}

func (lw *logWriter) write(line string) {
	line = strings.TrimRight(line, "\n")

	if len(line) <= 0 {
		return
	}

	lw.output(line)
}
