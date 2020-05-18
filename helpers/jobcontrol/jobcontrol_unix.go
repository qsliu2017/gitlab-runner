// +build darwin dragonfly freebsd linux netbsd openbsd

package jobcontrol

import (
	"syscall"
)

func (c *JobCmd) start() error {
	c.cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}
	return c.cmd.Start()
}

func (c *JobCmd) softKill() {
	syscall.Kill(-c.cmd.Process.Pid, syscall.SIGTERM)
}

func (c *JobCmd) hardKill() {
	syscall.Kill(-c.cmd.Process.Pid, syscall.SIGKILL)
}
