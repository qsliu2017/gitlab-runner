package process

import (
	"context"
	"net/url"
	"os/exec"
	"syscall"

	"github.com/pkg/errors"

	"gitlab.com/gitlab-org/ci-cd/tests/vault-ci-service/log"
)

const (
	CommandName = "vault"

	DefaultURL = "https://127.0.0.1:8200/"
)

type Process struct {
	cmd *exec.Cmd
}

func New(ctx context.Context, logger log.Logger, confFile string) (*Process, error) {
	p := new(Process)

	args := []string{
		"server",
		"-config",
		confFile,
	}

	p.cmd = exec.CommandContext(ctx, CommandName, args...)

	stdoutPipe, err := p.cmd.StdoutPipe()
	if err != nil {
		return nil, errors.Wrap(err, "couldn't create process stdout pipe")
	}

	stderrPipe, err := p.cmd.StderrPipe()
	if err != nil {
		return nil, errors.Wrap(err, "couldn't create process stdout pipe")
	}

	logger.NewStdoutWriter(CommandName, stdoutPipe)
	logger.NewStderrWriter(CommandName, stderrPipe)

	return p, nil
}

func (p *Process) Start() chan error {
	errCh := make(chan error, 1)

	go func() {
		errCh <- p.cmd.Run()
	}()

	return errCh
}

func (p *Process) Terminate() error {
	if p.cmd == nil {
		return nil
	}

	if p.cmd.Process == nil {
		return nil
	}

	return p.cmd.Process.Signal(syscall.SIGTERM)
}

func (p *Process) URL() (*url.URL, error) {
	return url.Parse(DefaultURL)
}
