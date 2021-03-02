package process

import (
	"context"
	"io"
	"os"
	"os/exec"
	"time"
)

type Commander interface {
	Start() error
	Wait() error
	Process() *os.Process
}

type CommandOptions struct {
	Dir string
	Env []string

	Stdout io.Writer
	Stderr io.Writer
	Stdin  io.Reader

	Logger Logger

	GracefulKillTimeout time.Duration
	ForceKillTimeout    time.Duration

	UseWindowsLegacyProcessStrategy bool
}

type osCmd struct {
	ctx      context.Context
	internal *exec.Cmd
	options  CommandOptions
	killer   KillWaiter
}

// NewOSCmd creates a new implementation of Commander using the os.Cmd from
// os/exec.
func NewOSCmd(ctx context.Context, executable string, args []string, options CommandOptions) Commander {
	c := exec.Command(executable, args...)
	c.Dir = options.Dir
	c.Env = options.Env
	c.Stdin = options.Stdin
	c.Stdout = options.Stdout
	c.Stderr = options.Stderr

	return &osCmd{
		ctx:      ctx,
		internal: c,
		options:  options,
		killer:   NewOSKillWait(options.Logger, options.GracefulKillTimeout, options.ForceKillTimeout),
	}
}

func (c *osCmd) Start() error {
	setProcessGroup(c.internal, c.options.UseWindowsLegacyProcessStrategy)

	return c.internal.Start()
}

func (c *osCmd) Wait() error {
	waitCh := make(chan error, 1)
	go func() {
		waitCh <- c.internal.Wait()
	}()

	select {
	case err := <-waitCh:
		return err
	case <-c.ctx.Done():
		return c.killer.KillAndWait(c, waitCh)
	}
}

func (c *osCmd) Process() *os.Process {
	return c.internal.Process
}
