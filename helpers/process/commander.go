package process

import (
	"io"
	"os"
	"os/exec"
	"syscall"
	"time"
)

type Commander interface {
	Start() error
	Wait() error
	Process() *os.Process
	SysProcAttr() *syscall.SysProcAttr
	SetSysProcAttr(attr *syscall.SysProcAttr)
}

type CommandOptions struct {
	Dir string
	Env []string

	Stdout io.Writer
	Stderr io.Writer

	Logger Logger

	GracefulKillTimeout time.Duration
	ForceKillTimeout    time.Duration
}

type cmd struct {
	internal *exec.Cmd
}

var NewCmd = func(executable string, args []string, options CommandOptions) Commander {
	c := exec.Command(executable, args...)
	c.Dir = options.Dir
	c.Env = options.Env
	c.Stdin = nil
	c.Stdout = options.Stdout
	c.Stderr = options.Stderr

	return &cmd{internal: c}
}

func (c *cmd) Start() error {
	return c.internal.Start()
}

func (c *cmd) Wait() error {
	return c.internal.Wait()
}

func (c *cmd) Process() *os.Process {
	return c.internal.Process
}

func (c *cmd) SysProcAttr() *syscall.SysProcAttr {
	return c.internal.SysProcAttr
}

func (c *cmd) SetSysProcAttr(attr *syscall.SysProcAttr) {
	c.internal.SysProcAttr = attr
}
