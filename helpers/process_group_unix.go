// +build darwin dragonfly freebsd linux netbsd openbsd

package helpers

import (
	"os/exec"
	"syscall"
)

func SetProcessGroup(cmd *exec.Cmd) {
	// Create process group
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}
}

func KillProcessGroup(cmd *exec.Cmd) {
	killUnixProcessGroup(cmd, syscall.SIGTERM)
}

func LegacyKillProcessGroup(cmd *exec.Cmd) {
	killUnixProcessGroup(cmd, syscall.SIGKILL)
}

func killUnixProcessGroup(cmd *exec.Cmd, signal syscall.Signal) {
	if cmd == nil {
		return
	}

	process := cmd.Process
	if process != nil {
		/* The process spawned for the job shall be responsible for
		 * propagating signals to any children it may have, and
		 * likewise for each of those child processes.
		 */
		if process.Pid > 0 {
			_ = syscall.Kill(-process.Pid, signal)
		} else {
			// doing normal kill
			_ = process.Signal(signal)
		}
	}
}
