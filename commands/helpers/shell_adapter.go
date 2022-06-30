package helpers

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"

	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/process"
	"gitlab.com/gitlab-org/gitlab-runner/log"
)

type ShellAdapterCommand struct {
	CommandLine string `long:"command-line"`
}

func (c *ShellAdapterCommand) Execute(_ *cli.Context) {
	ctx, cancelFn := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancelFn()

	l := logrus.New()
	l.SetFormatter(new(log.RunnerTextFormatter))
	logger := l.WithFields(logrus.Fields{
		"pid":     os.Getpid(),
		"context": "shell-adapter",
	})

	commandLine := strings.SplitN(c.CommandLine, " ", 2)

	cmdOpts := process.CommandOptions{
		Env:    os.Environ(),
		Stdin:  os.Stdin,
		Stdout: os.Stdout,
		Stderr: os.Stderr,
	}

	cmd := process.NewOSCmd(commandLine[0], strings.Split(commandLine[1], " "), cmdOpts)
	err := cmd.Start()
	if err != nil {
		logger.WithError(err).Fatal("Starting shell command")
	}

	waitErrCh := make(chan error)
	go func() {
		waitErrCh <- cmd.Wait()
	}()

	select {
	case err = <-waitErrCh:
		logExit(logger, err, "Shell command exited")
	case <-ctx.Done():
		logger.Warn("Termination signal received; exiting")

		err = process.
			NewOSKillWait(newLoggerAdapter(logger), process.GracefulTimeout, process.KillTimeout).
			KillAndWait(cmd, waitErrCh)

		logExit(logger, err, "Killing shell command")
	}
}

func logExit(logger logrus.FieldLogger, err error, msg string) {
	exitLogger := logger.Info
	if err != nil {
		exitLogger = logger.WithError(err).Fatal

		var ee *exec.ExitError
		if errors.As(err, &ee) {
			exitLogger = logger.WithField("state", ee.ProcessState).Info
		}
	}

	exitLogger(msg)
}

type loggerAdapter struct {
	internal logrus.FieldLogger
}

func newLoggerAdapter(l logrus.FieldLogger) *loggerAdapter {
	return &loggerAdapter{internal: l}
}

func (l *loggerAdapter) WithFields(fields logrus.Fields) process.Logger {
	l.internal = l.internal.WithFields(fields)

	return l
}

func (l *loggerAdapter) Warn(args ...interface{}) {
	l.internal.Warn(args...)
}

func init() {
	common.RegisterCommand2(
		"shell-adapter",
		"(Experimental) Adapter supporting process termination on *unix when Shell executor is used",
		new(ShellAdapterCommand),
	)
}
