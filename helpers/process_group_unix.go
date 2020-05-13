// +build darwin dragonfly freebsd linux netbsd openbsd

package helpers

import (
	"os/exec"
	"syscall"
)

// ProcessGroupKiller configures exec.Cmd and returns a function for killing
// the process.
func ProcessGroupKiller(cmd *exec.Cmd) func() {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}

	return func() {
		if cmd.Process == nil {
			return
		}

		if cmd.Process.Pid > 0 {
			syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		} else {
			cmd.Process.Kill()
		}
	}
}
