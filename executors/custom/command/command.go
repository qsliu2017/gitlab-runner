package command

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strconv"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/executors/custom/api"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/process"
)

const (
	BuildFailureExitCode  = 1
	SystemFailureExitCode = 2
)

type Command interface {
	Run() error
}

var newCommander = process.NewOSCmd

type command struct {
	cmd process.Commander
}

func New(ctx context.Context, executable string, args []string, options process.CommandOptions) Command {
	defaultVariables := map[string]string{
		"TMPDIR":                          options.Dir,
		api.BuildFailureExitCodeVariable:  strconv.Itoa(BuildFailureExitCode),
		api.SystemFailureExitCodeVariable: strconv.Itoa(SystemFailureExitCode),
	}

	env := os.Environ()
	for key, value := range defaultVariables {
		env = append(env, fmt.Sprintf("%s=%s", key, value))
	}
	options.Env = append(env, options.Env...)

	return &command{
		cmd: newCommander(ctx, executable, args, options),
	}
}

func (c *command) Run() error {
	err := c.cmd.Start()
	if err != nil {
		return fmt.Errorf("failed to start command: %w", err)
	}

	err = c.cmd.Wait()

	eerr, ok := err.(*exec.ExitError)
	if ok {
		exitCode := getExitCode(eerr)
		switch {
		case exitCode == BuildFailureExitCode:
			err = &common.BuildError{Inner: eerr, ExitCode: exitCode}
		case exitCode != SystemFailureExitCode:
			err = &ErrUnknownFailure{Inner: eerr, ExitCode: exitCode}
		}
	}

	return err
}

var getExitCode = func(err *exec.ExitError) int {
	return err.ExitCode()
}
