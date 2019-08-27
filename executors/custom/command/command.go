package command

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"syscall"
	"time"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/process"
)

const (
	BuildFailureExitCode  = 1
	SystemFailureExitCode = 2

	BuildFailureExitCodeVariable  = "BUILD_FAILURE_EXIT_CODE"
	SystemFailureExitCodeVariable = "SYSTEM_FAILURE_EXIT_CODE"
)

type Command interface {
	Run() error
}

var newProcessKillWaiter = process.NewKillWaiter

type command struct {
	context context.Context
	cmd     process.Commander

	waitCh chan error

	logger process.Logger

	gracefulKillTimeout time.Duration
	forceKillTimeout    time.Duration
}

func New(ctx context.Context, executable string, args []string, options process.CommandOptions) Command {
	defaultVariables := map[string]string{
		"TMPDIR":                      options.Dir,
		BuildFailureExitCodeVariable:  strconv.Itoa(BuildFailureExitCode),
		SystemFailureExitCodeVariable: strconv.Itoa(SystemFailureExitCode),
	}

	env := os.Environ()
	for key, value := range defaultVariables {
		env = append(env, fmt.Sprintf("%s=%s", key, value))
	}
	options.Env = append(env, options.Env...)

	return &command{
		context:             ctx,
		cmd:                 process.NewCmd(executable, args, options),
		waitCh:              make(chan error),
		logger:              options.Logger,
		gracefulKillTimeout: options.GracefulKillTimeout,
		forceKillTimeout:    options.ForceKillTimeout,
	}
}

func (c *command) Run() error {
	err := c.cmd.Start()
	if err != nil {
		return fmt.Errorf("failed to start command: %v", err)
	}

	go c.waitForCommand()

	select {
	case err = <-c.waitCh:
		return err

	case <-c.context.Done():
		return newProcessKillWaiter(c.logger, c.gracefulKillTimeout, c.forceKillTimeout).
			KillAndWait(c.cmd, c.waitCh)
	}
}

var getExitStatus = func(err *exec.ExitError) int {
	// TODO: simplify when we will update to Go 1.12. ExitStatus()
	//       is available there directly from err.Sys().
	return err.Sys().(syscall.WaitStatus).ExitStatus()
}

func (c *command) waitForCommand() {
	err := c.cmd.Wait()

	eerr, ok := err.(*exec.ExitError)
	if ok {
		exitCode := getExitStatus(eerr)
		if exitCode == BuildFailureExitCode {
			err = &common.BuildError{Inner: eerr}
		} else if exitCode != SystemFailureExitCode {
			err = &ErrUnknownFailure{Inner: eerr, ExitCode: exitCode}
		}
	}

	c.waitCh <- err
}
